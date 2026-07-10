package correlator

import (
	"github.com/zerotrace/zerotrace/agent/reader"
	proto "github.com/zerotrace/zerotrace/proto"
)

// TCPEventInput is a test-friendly struct that wraps TCPEvent fields
// using plain strings for addresses instead of [4]byte arrays.
type TCPEventInput struct {
	PID         uint32
	TID         uint32
	TimestampNs uint64
	Bytes       uint64
	Sport       uint16
	Dport       uint16
	EventType   uint8
	SAddrStr    [16]byte // plain string stored as bytes for test convenience
	DAddrStr    [16]byte
}

// HandleTCPEvent accepts a TCPEventInput (test-friendly) and calls the
// internal correlator logic.
func (c *Correlator) HandleTCPEventFromInput(ev *TCPEventInput) {
	sip := nullTermStr(ev.SAddrStr[:])
	dip := nullTermStr(ev.DAddrStr[:])
	dport := swapPort(ev.Dport)
	sport := ev.Sport

	if sip == "" {
		sip = "0.0.0.0"
	}
	if dip == "" {
		dip = "0.0.0.0"
	}

	key := connKey(sip, dip, sport, dport)

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[key]
	if !exists {
		conn = &ConnectionState{
			TraceID:      newTraceID(),
			SpanID:       newSpanID(),
			PID:          ev.PID,
			TID:          ev.TID,
			SourceAddr:   sip,
			DestAddr:     dip,
			SourcePort:   sport,
			DestPort:     dport,
			FirstEventNs: ev.TimestampNs,
		}
		c.connections[key] = conn
	}
	conn.LastEventNs = ev.TimestampNs
	if ev.EventType == reader.EventTCPSend {
		conn.BytesSent += ev.Bytes
	} else {
		conn.BytesRecv += ev.Bytes
	}
}

// ForceFlushAll emits all connections as spans regardless of age.
// Used in tests to drain the correlator without waiting 30 seconds.
func (c *Correlator) ForceFlushAll(spanCh chan<- *proto.Span) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, conn := range c.connections {
		span := c.buildSpan(conn)
		select {
		case spanCh <- span:
		default:
		}
		delete(c.connections, key)
	}
}

// ExtractHTTPRequest is exported for testing purposes.
// It wraps the internal extractHTTPRequest function.
func ExtractHTTPRequest(data []byte) (method, path string) {
	return extractHTTPRequest(data)
}

// nullTermStr extracts a null-terminated string from a byte slice.
func nullTermStr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}


// TCPEventInput convenience constructor.
func MakeTCPEvent(pid, tid uint32, sip, dip string, sport, dport uint16, byts uint32, ts uint64, isSend bool) *TCPEventInput {
	ev := &TCPEventInput{
		PID:         pid,
		TID:         tid,
		TimestampNs: ts,
		Bytes:       uint64(byts),
		Sport:       sport,
		Dport:       dport,
	}
	copy(ev.SAddrStr[:], []byte(sip))
	copy(ev.DAddrStr[:], []byte(dip))
	if isSend {
		ev.EventType = reader.EventTCPSend
	} else {
		ev.EventType = reader.EventTCPRecv
	}
	return ev
}
