package correlator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/zerotrace/zerotrace/agent/enricher"
	"github.com/zerotrace/zerotrace/agent/reader"
	proto "github.com/zerotrace/zerotrace/proto"
)

// ---------------------------------------------------------------------------
// ID generation
// ---------------------------------------------------------------------------

// newHexID generates a cryptographically random hex string of n bytes.
func newHexID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use time-based pseudo-random (should never happen)
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func newTraceID() string { return newHexID(16) } // 32 hex chars
func newSpanID() string  { return newHexID(8) }  // 16 hex chars

// ---------------------------------------------------------------------------
// Connection state
// ---------------------------------------------------------------------------

// ConnectionState tracks a single TCP connection lifetime.
type ConnectionState struct {
	TraceID        string
	SpanID         string
	ParentSpanID   string
	PID            uint32
	TID            uint32
	Comm           string
	SourceAddr     string
	DestAddr       string
	SourcePort     uint16
	DestPort       uint16
	BytesSent      uint64
	BytesRecv      uint64
	FirstEventNs   uint64
	LastEventNs    uint64
	IsTLS          bool
	HTTPMethod     string
	HTTPPath       string
	HTTPStatusCode int
}

// connKey builds a unique key for a TCP connection using all 4-tuple fields.
// Ports are byte-swapped (kernel stores dport in big-endian).
func connKey(sip, dip string, sport, dport uint16) string {
	return fmt.Sprintf("%s:%d→%s:%d", sip, sport, dip, dport)
}

// ---------------------------------------------------------------------------
// Process cache
// ---------------------------------------------------------------------------

type processInfo struct {
	Comm     string
	Filename string
}

// ---------------------------------------------------------------------------
// Correlator
// ---------------------------------------------------------------------------

// Correlator assembles eBPF events into spans.
type Correlator struct {
	log          *zap.Logger
	mu           sync.RWMutex
	connections  map[string]*ConnectionState
	processes    map[uint32]*processInfo
	// sslPIDToConn maps pid→activeConnKey for TLS association
	sslPIDToConn map[uint32]string
	activePIDs   map[uint32]*ConnectionState
}

// NewCorrelator creates a new Correlator.
func NewCorrelator(log *zap.Logger) *Correlator {
	return &Correlator{
		log:          log,
		connections:  make(map[string]*ConnectionState),
		processes:    make(map[uint32]*processInfo),
		sslPIDToConn: make(map[uint32]string),
		activePIDs:   make(map[uint32]*ConnectionState),
	}
}

// ---------------------------------------------------------------------------
// TCP event handling
// ---------------------------------------------------------------------------

// HandleTCPEvent processes a raw TCP send/recv event from the kernel.
func (c *Correlator) HandleTCPEvent(ev *reader.TCPEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sip := reader.Addr4String(ev.Saddr)
	dip := reader.Addr4String(ev.Daddr)
	// dport from kernel is big-endian; swap it
	dport := swapPort(ev.Dport)
	sport := ev.Sport

	// Skip loopback self-connections
	if sip == "0.0.0.0" && dip == "0.0.0.0" {
		return
	}

	key := connKey(sip, dip, sport, dport)
	reverseKey := connKey(dip, sip, dport, sport)

	conn, ok := c.connections[key]
	if !ok {
		// New connection
		conn = &ConnectionState{
			TraceID:      newTraceID(),
			SpanID:       newSpanID(),
			PID:          ev.PID,
			TID:          ev.TID,
			Comm:         reader.NullTermString(ev.Comm[:]),
			SourceAddr:   sip,
			DestAddr:     dip,
			SourcePort:   sport,
			DestPort:     dport,
			FirstEventNs: ev.TimestampNs,
		}

		// Cross-process linking for localhost testing
		if revConn, revExists := c.connections[reverseKey]; revExists {
			if sport < 32768 {
				// We are the server, link to client
				conn.TraceID = revConn.TraceID
				conn.ParentSpanID = revConn.SpanID
				c.log.Info("cross-process link (server)", zap.Uint32("pid", ev.PID), zap.String("trace_id", conn.TraceID))
			} else {
				// We are the client, server already exists? Link server to us
				revConn.TraceID = conn.TraceID
				revConn.ParentSpanID = conn.SpanID
				c.log.Info("cross-process link (client)", zap.Uint32("pid", ev.PID), zap.String("trace_id", conn.TraceID))
			}
		}

		c.connections[key] = conn
		// Record which connection this PID is using (for SSL association)
		c.sslPIDToConn[ev.PID] = key
		c.log.Debug("new connection",
			zap.String("key", key),
			zap.String("comm", conn.Comm),
			zap.Uint32("pid", ev.PID),
		)
	}

	// Always try to link on TCPSend if not linked yet
	if ev.EventType == reader.EventTCPSend && conn.ParentSpanID == "" {
		if activeConn, ok := c.activePIDs[ev.PID]; ok && activeConn != conn {
			if (ev.TimestampNs - activeConn.LastEventNs) < 2000000000 {
				conn.TraceID = activeConn.TraceID
				conn.ParentSpanID = activeConn.SpanID
				c.log.Info("linked span", zap.Uint32("pid", ev.PID), zap.String("comm", reader.CommString(ev.Comm)), zap.String("trace_id", conn.TraceID))
			} else {
				c.log.Info("did not link span, timeout", zap.Uint32("pid", ev.PID), zap.String("comm", reader.CommString(ev.Comm)), zap.Uint64("diff", ev.TimestampNs - activeConn.LastEventNs))
			}
		} else {
			c.log.Info("did not link span, no active pid", zap.Uint32("pid", ev.PID), zap.String("comm", reader.CommString(ev.Comm)))
		}
	}

	conn.LastEventNs = ev.TimestampNs
	if ev.EventType == reader.EventTCPSend {
		conn.BytesSent += uint64(ev.Bytes)
	} else {
		conn.BytesRecv += uint64(ev.Bytes)
		if conn.ParentSpanID != "" || conn.SourcePort < 32768 {
			c.activePIDs[ev.PID] = conn
			if conn.ParentSpanID == "" {
				c.log.Info("set active pid (root)", zap.Uint32("pid", ev.PID), zap.String("comm", conn.Comm), zap.String("trace_id", conn.TraceID))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// SSL event handling
// ---------------------------------------------------------------------------

// HandleSSLEvent processes a TLS plaintext intercept event.
func (c *Correlator) HandleSSLEvent(ev *reader.SSLEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Mark the associated TCP connection as TLS
	connKey, ok := c.sslPIDToConn[ev.PID]
	if ok {
		if conn, exists := c.connections[connKey]; exists {
			conn.IsTLS = true
			conn.LastEventNs = ev.TimestampNs

			// Attempt to parse HTTP method/path from plaintext data
			if ev.EventType == reader.EventSSLWrite {
				method, path := extractHTTPRequest(ev.Data[:])
				if method != "" {
					conn.HTTPMethod = method
					conn.HTTPPath = path
				}
			} else if ev.EventType == reader.EventSSLRead {
				code := extractHTTPStatusCode(ev.Data[:])
				if code > 0 {
					conn.HTTPStatusCode = code
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Process event handling
// ---------------------------------------------------------------------------

// HandleProcEvent processes a process exec/exit event.
func (c *Correlator) HandleProcEvent(ev *reader.ProcEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	comm := reader.NullTermString(ev.Comm[:])
	filename := reader.NullTermString(ev.Filename[:])

	switch ev.EventType {
	case reader.EventProcExec:
		c.processes[ev.PID] = &processInfo{Comm: comm, Filename: filename}
		c.log.Debug("proc exec", zap.String("comm", comm), zap.Uint32("pid", ev.PID))
	case reader.EventProcExit:
		delete(c.processes, ev.PID)
		// Clean up SSL mapping
		delete(c.sslPIDToConn, ev.PID)
		c.log.Debug("proc exit", zap.String("comm", comm), zap.Uint32("pid", ev.PID))
	}
}

// ---------------------------------------------------------------------------
// Span flushing
// ---------------------------------------------------------------------------

// FlushOldConnections emits spans for connections idle longer than the threshold,
// then removes them from the connection map.
func (c *Correlator) FlushOldConnections(spanCh chan<- *proto.Span) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := uint64(time.Now().UnixNano())
	threshold := uint64(30 * time.Second)

	for key, conn := range c.connections {
		if now-conn.LastEventNs > threshold {
			span := c.buildSpan(conn)
			select {
			case spanCh <- span:
			default:
				c.log.Warn("span channel full, dropping span", zap.String("trace_id", conn.TraceID))
			}
			delete(c.connections, key)
			// Remove SSL mapping if it still points here
			if k, ok := c.sslPIDToConn[conn.PID]; ok && k == key {
				delete(c.sslPIDToConn, conn.PID)
			}
		}
	}

	for pid, activeConn := range c.activePIDs {
		found := false
		for _, cConn := range c.connections {
			if cConn == activeConn {
				found = true
				break
			}
		}
		if !found {
			delete(c.activePIDs, pid)
		}
	}
}

// buildSpan converts a ConnectionState into a proto.Span.
func (c *Correlator) buildSpan(conn *ConnectionState) *proto.Span {
	opName := "tcp.connection"
	if conn.HTTPMethod != "" {
		opName = conn.HTTPMethod + " " + conn.HTTPPath
	} else if conn.IsTLS {
		opName = "tls.connection"
	}

	tags := map[string]string{
		"peer.address":   fmt.Sprintf("%s:%d", conn.DestAddr, conn.DestPort),
		"local.address":  fmt.Sprintf("%s:%d", conn.SourceAddr, conn.SourcePort),
		"process.pid":    fmt.Sprintf("%d", conn.PID),
		"process.comm":   conn.Comm,
		"bytes.sent":     fmt.Sprintf("%d", conn.BytesSent),
		"bytes.received": fmt.Sprintf("%d", conn.BytesRecv),
		"tls":            fmt.Sprintf("%v", conn.IsTLS),
	}
	if conn.HTTPStatusCode > 0 {
		tags["http.status_code"] = fmt.Sprintf("%d", conn.HTTPStatusCode)
	}

	kind := proto.SpanKind_SPAN_KIND_CLIENT
	if conn.BytesRecv > conn.BytesSent {
		kind = proto.SpanKind_SPAN_KIND_SERVER
	}

	return &proto.Span{
		TraceId:       conn.TraceID,
		SpanId:        conn.SpanID,
		ParentSpanId:  conn.ParentSpanID,
		ServiceName:   conn.Comm,
		OperationName: opName,
		StartTimeNs:   enricher.MonotonicToUnixNs(conn.FirstEventNs),
		EndTimeNs:     enricher.MonotonicToUnixNs(conn.LastEventNs),
		Tags:          tags,
		Kind:          kind,
	}
}

// ---------------------------------------------------------------------------
// HTTP parsing helpers
// ---------------------------------------------------------------------------

var httpMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT"}

// extractHTTPRequest attempts to parse an HTTP request line from raw TLS plaintext.
// Returns method and path, or empty strings if not HTTP.
func extractHTTPRequest(data []byte) (method, path string) {
	// Find first null or end
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		end = len(data)
	}
	line := string(data[:end])

	for _, m := range httpMethods {
		if strings.HasPrefix(line, m+" ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 {
				// path may have query string — keep as-is
				pathPart := parts[1]
				if idx := strings.IndexByte(pathPart, '\n'); idx >= 0 {
					pathPart = pathPart[:idx]
				}
				pathPart = strings.TrimSpace(pathPart)
				return m, pathPart
			}
		}
	}
	return "", ""
}

// extractHTTPStatusCode attempts to parse an HTTP/1.x status code from response data.
func extractHTTPStatusCode(data []byte) int {
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		end = len(data)
	}
	if end < 12 {
		return 0
	}
	line := string(data[:end])
	if !strings.HasPrefix(line, "HTTP/") {
		return 0
	}
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return 0
	}
	code := 0
	fmt.Sscanf(parts[1], "%d", &code)
	return code
}

// swapPort converts a big-endian port (as stored by the kernel) to host byte order.
func swapPort(p uint16) uint16 {
	return (p>>8)&0xff | (p&0xff)<<8
}

// ---------------------------------------------------------------------------
// Run loop
// ---------------------------------------------------------------------------

// Run is the main event dispatch loop for the correlator. It reads typed events
// from eventCh and routes them to the appropriate handler. Every 5 seconds it
// flushes idle connections to spanCh.
func Run(ctx context.Context, eventCh <-chan reader.RawEvent, spanCh chan<- *proto.Span, log *zap.Logger) {
	corr := NewCorrelator(log)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			corr.log.Info("shutting down correlator, flushing all remaining connections")
			corr.mu.Lock()
			for key, conn := range corr.connections {
				span := corr.buildSpan(conn)
				select {
				case spanCh <- span:
				default:
					corr.log.Warn("span channel full, dropping span during shutdown", zap.String("trace_id", conn.TraceID))
				}
				delete(corr.connections, key)
			}
			corr.mu.Unlock()
			return

		case ev := <-eventCh:
			switch ev.Kind {
			case reader.EventTCPSend, reader.EventTCPRecv:
				if ev.TCP != nil {
					corr.HandleTCPEvent(ev.TCP)
				}
			case reader.EventSSLWrite, reader.EventSSLRead:
				if ev.SSL != nil {
					corr.HandleSSLEvent(ev.SSL)
				}
			case reader.EventProcExec, reader.EventProcExit:
				if ev.Proc != nil {
					corr.HandleProcEvent(ev.Proc)
				}
			default:
				log.Debug("unknown event kind", zap.Uint8("kind", ev.Kind))
			}

		case <-ticker.C:
			corr.FlushOldConnections(spanCh)
		}
	}
}
