package reader

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// minEventSize is the minimum number of bytes needed to determine event type.
// All events start with: timestamp_ns(8) + pid(4) + tid(4) = 16 bytes before diverging.
// The event_type field position varies per struct, so we use struct-specific sizes.
const (
	// Exact sizes from common.h structs (packed, LE)
	tcpEventSize  = 57 // 8+4+4+8+4+2+2+4+4+1+16 = 57 (kernel may add 7 pad → 64)
	sslEventSize  = 297 // 8+4+4+8+4+1+16+256 = 301 (check alignment)
	procEventSize = 161 // 8+4+4+1+16+128+4 = 165

	// Minimum safe buffer to read event_type
	minBufSize = 17
)

var le = binary.LittleEndian

// ParseEvent inspects the first bytes of data to determine event kind,
// then fully deserializes the appropriate struct.
// Returns a populated RawEvent or an error if data is malformed.
func ParseEvent(data []byte) (*RawEvent, error) {
	if len(data) < minBufSize {
		return nil, fmt.Errorf("event too small: %d bytes", len(data))
	}

	// Determine event type: for tcp_event_t, event_type is at offset 40.
	// But we need a universal approach: read the first struct field discriminator.
	// Strategy: try each struct size, use the event_type from each known offset.
	// Since all events share the ring buffer, the BPF program puts one event type
	// per submission. We read event_type from offset 40 (tcp), 33 (ssl), 16 (proc).
	// Use a heuristic: try tcp first (most common), then probe other offsets.

	evType, err := probeEventType(data)
	if err != nil {
		return nil, err
	}

	ev := &RawEvent{Kind: evType, Raw: data}

	switch evType {
	case EventTCPSend, EventTCPRecv:
		t, err := parseTCPEvent(data)
		if err != nil {
			return nil, err
		}
		ev.TCP = t
	case EventSSLWrite, EventSSLRead:
		s, err := parseSSLEvent(data)
		if err != nil {
			return nil, err
		}
		ev.SSL = s
	case EventProcExec, EventProcExit:
		p, err := parseProcEvent(data)
		if err != nil {
			return nil, err
		}
		ev.Proc = p
	default:
		return nil, fmt.Errorf("unknown event type: %d", evType)
	}

	return ev, nil
}

// probeEventType determines event type by checking candidate offsets.
// The offsets are derived from C struct layouts (packed, x86_64).
//
// tcp_event_t  → event_type at byte offset 40
// ssl_event_t  → event_type at byte offset 32
// proc_event_t → event_type at byte offset 16
func probeEventType(data []byte) (uint8, error) {
	candidates := []int{40, 32, 16}
	for _, off := range candidates {
		if len(data) <= off {
			continue
		}
		t := data[off]
		if t >= EventTCPSend && t <= EventHTTPResp {
			return t, nil
		}
	}
	return 0, fmt.Errorf("cannot determine event type from %d bytes", len(data))
}

// parseTCPEvent deserializes a tcp_event_t.
// Layout (LE):
//
//	offset 0:  uint64 timestamp_ns
//	offset 8:  uint32 pid
//	offset 12: uint32 tid
//	offset 16: uint64 sk_ptr
//	offset 24: uint32 bytes
//	offset 28: uint16 sport
//	offset 30: uint16 dport
//	offset 32: [4]uint8 saddr
//	offset 36: [4]uint8 daddr
//	offset 40: uint8  event_type
//	offset 41: [16]byte comm
func parseTCPEvent(data []byte) (*TCPEvent, error) {
	const need = 57
	if len(data) < need {
		return nil, fmt.Errorf("tcp event: need %d bytes, got %d", need, len(data))
	}
	r := bytes.NewReader(data)
	ev := &TCPEvent{}
	fields := []interface{}{
		&ev.TimestampNs,
		&ev.PID,
		&ev.TID,
		&ev.SkPtr,
		&ev.Bytes,
		&ev.Sport,
		&ev.Dport,
		&ev.Saddr,
		&ev.Daddr,
		&ev.EventType,
		&ev.Comm,
	}
	for _, f := range fields {
		if err := binary.Read(r, le, f); err != nil {
			return nil, fmt.Errorf("tcp event parse: %w", err)
		}
	}
	return ev, nil
}

// parseSSLEvent deserializes an ssl_event_t.
// Layout (LE):
//
//	offset 0:  uint64 timestamp_ns
//	offset 8:  uint32 pid
//	offset 12: uint32 tid
//	offset 16: uint64 ssl_ptr
//	offset 24: uint32 bytes
//	offset 28: uint8  event_type   ← note: 3 bytes struct pad before comm to align
//	offset 29: [16]byte comm       (but C packs without explicit align, so 29)
//	offset 45: [256]byte data
func parseSSLEvent(data []byte) (*SSLEvent, error) {
	const need = 301
	if len(data) < need {
		return nil, fmt.Errorf("ssl event: need %d bytes, got %d", need, len(data))
	}
	r := bytes.NewReader(data)
	ev := &SSLEvent{}
	fields := []interface{}{
		&ev.TimestampNs,
		&ev.PID,
		&ev.TID,
		&ev.SSLPtr,
		&ev.Bytes,
		&ev.EventType,
		&ev.Comm,
		&ev.Data,
	}
	for _, f := range fields {
		if err := binary.Read(r, le, f); err != nil {
			return nil, fmt.Errorf("ssl event parse: %w", err)
		}
	}
	return ev, nil
}

// parseProcEvent deserializes a proc_event_t.
// Layout (LE):
//
//	offset 0:  uint64 timestamp_ns
//	offset 8:  uint32 pid
//	offset 12: uint32 ppid
//	offset 16: uint8  event_type
//	offset 17: [16]byte comm
//	offset 33: [128]byte filename
//	offset 161: uint32 exit_code
func parseProcEvent(data []byte) (*ProcEvent, error) {
	const need = 165
	if len(data) < need {
		return nil, fmt.Errorf("proc event: need %d bytes, got %d", need, len(data))
	}
	r := bytes.NewReader(data)
	ev := &ProcEvent{}
	fields := []interface{}{
		&ev.TimestampNs,
		&ev.PID,
		&ev.PPID,
		&ev.EventType,
		&ev.Comm,
		&ev.Filename,
		&ev.ExitCode,
	}
	for _, f := range fields {
		if err := binary.Read(r, le, f); err != nil {
			return nil, fmt.Errorf("proc event parse: %w", err)
		}
	}
	return ev, nil
}

// NullTermString converts a null-terminated byte array to a Go string.
func NullTermString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
