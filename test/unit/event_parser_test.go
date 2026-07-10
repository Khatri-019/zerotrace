package unit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/zerotrace/zerotrace/agent/reader"
)

var le = binary.LittleEndian

// buildTCPEventBytes creates a byte slice matching the tcp_event_t layout.
func buildTCPEventBytes(ts uint64, pid, tid uint32, skPtr uint64, byts uint32,
	sport, dport uint16, saddr, daddr [4]byte, evType uint8, comm string) []byte {

	buf := &bytes.Buffer{}
	write := func(v interface{}) { binary.Write(buf, le, v) } //nolint:errcheck
	write(ts)
	write(pid)
	write(tid)
	write(skPtr)
	write(byts)
	write(sport)
	write(dport)
	buf.Write(saddr[:])
	buf.Write(daddr[:])
	write(evType)
	// comm: exactly 16 bytes, null padded
	commBytes := [16]byte{}
	copy(commBytes[:], comm)
	buf.Write(commBytes[:])
	return buf.Bytes()
}

// TestEventParserTCPEvent checks that binary deserialization produces correct field values.
func TestEventParserTCPEvent(t *testing.T) {
	saddr := [4]byte{10, 0, 0, 1}
	daddr := [4]byte{10, 0, 0, 2}
	data := buildTCPEventBytes(
		1_000_000_000, // timestamp_ns
		42, 43,        // pid, tid
		0xDEADBEEF,    // sk_ptr
		512,           // bytes
		1234, 80,      // sport, dport
		saddr, daddr,
		1, // EVENT_TCP_SEND
		"myapp",
	)

	ev, err := reader.ParseEvent(data)
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if ev.Kind != reader.EventTCPSend {
		t.Errorf("expected kind=%d, got %d", reader.EventTCPSend, ev.Kind)
	}
	if ev.TCP == nil {
		t.Fatal("expected TCP field to be set")
	}
	tcp := ev.TCP
	if tcp.TimestampNs != 1_000_000_000 {
		t.Errorf("timestamp mismatch: %d", tcp.TimestampNs)
	}
	if tcp.PID != 42 || tcp.TID != 43 {
		t.Errorf("pid/tid mismatch: %d/%d", tcp.PID, tcp.TID)
	}
	if tcp.Bytes != 512 {
		t.Errorf("bytes mismatch: %d", tcp.Bytes)
	}
	if tcp.EventType != reader.EventTCPSend {
		t.Errorf("event_type mismatch: %d", tcp.EventType)
	}
	comm := reader.NullTermString(tcp.Comm[:])
	if comm != "myapp" {
		t.Errorf("comm mismatch: %q", comm)
	}
}

// TestEventParserTooSmall verifies rejection of undersized buffers.
func TestEventParserTooSmall(t *testing.T) {
	_, err := reader.ParseEvent([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for small buffer, got nil")
	}
}

// TestEventParserUnknownType verifies rejection of unrecognised event types.
func TestEventParserUnknownType(t *testing.T) {
	// Build a buffer with event_type=99 at offset 40
	data := make([]byte, 64)
	data[40] = 99 // invalid type at tcp_event_t offset
	data[32] = 99 // and at ssl_event_t offset
	data[16] = 99 // and at proc_event_t offset
	_, err := reader.ParseEvent(data)
	if err == nil {
		t.Error("expected error for unknown event type, got nil")
	}
}

// TestNullTermString checks null-termination stripping.
func TestNullTermString(t *testing.T) {
	cases := []struct {
		in  []byte
		out string
	}{
		{[]byte("hello\x00world"), "hello"},
		{[]byte("noterm"), "noterm"},
		{[]byte{}, ""},
		{[]byte("\x00"), ""},
	}
	for _, c := range cases {
		got := reader.NullTermString(c.in)
		if got != c.out {
			t.Errorf("NullTermString(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}
