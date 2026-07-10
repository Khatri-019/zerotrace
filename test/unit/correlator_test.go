package unit

import (
	"testing"

	"go.uber.org/zap"

	"github.com/zerotrace/zerotrace/agent/correlator"
	proto "github.com/zerotrace/zerotrace/proto"
)

func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	l, _ := zap.NewDevelopment()
	return l
}

func drainSpanCh(ch chan *proto.Span) []*proto.Span {
	close(ch)
	var result []*proto.Span
	for sp := range ch {
		result = append(result, sp)
	}
	return result
}

// TestCorrelatorConnKeyUniqueness ensures connections with different ports
// produce distinct spans (original bug: connKey() ignored ports → collisions).
func TestCorrelatorConnKeyUniqueness(t *testing.T) {
	c := correlator.NewCorrelator(newTestLogger(t))
	spanCh := make(chan *proto.Span, 10)

	// Connection 1: sport=1000, dport=80
	c.HandleTCPEventFromInput(correlator.MakeTCPEvent(1, 1, "10.0.0.1", "10.0.0.2", 1000, 80, 100, 1000, true))
	// Connection 2: sport=1001, dport=8080 — different ports → different key
	c.HandleTCPEventFromInput(correlator.MakeTCPEvent(2, 2, "10.0.0.1", "10.0.0.2", 1001, 8080, 200, 2000, true))

	c.ForceFlushAll(spanCh)
	spans := drainSpanCh(spanCh)

	if len(spans) != 2 {
		t.Errorf("expected 2 spans (distinct connections), got %d", len(spans))
	}
}

// TestCorrelatorByteAccumulation checks that bytes are summed across events.
func TestCorrelatorByteAccumulation(t *testing.T) {
	c := correlator.NewCorrelator(newTestLogger(t))
	spanCh := make(chan *proto.Span, 10)

	// 3 TCP sends on the same connection (same 4-tuple)
	for i := uint64(0); i < 3; i++ {
		c.HandleTCPEventFromInput(correlator.MakeTCPEvent(1, 1, "10.0.0.1", "10.0.0.2", 5000, 80, 512, (i+1)*1_000_000_000, true))
	}

	c.ForceFlushAll(spanCh)
	spans := drainSpanCh(spanCh)

	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	got := spans[0].Tags["bytes.sent"]
	if got != "1536" { // 3 × 512
		t.Errorf("expected bytes.sent=1536, got %q", got)
	}
}

// TestCorrelatorSpanIDFormat checks that span and trace IDs are 16/32 lowercase hex chars.
func TestCorrelatorSpanIDFormat(t *testing.T) {
	c := correlator.NewCorrelator(newTestLogger(t))
	spanCh := make(chan *proto.Span, 10)

	c.HandleTCPEventFromInput(correlator.MakeTCPEvent(1, 1, "192.168.1.1", "10.0.0.2", 2000, 443, 64, 1_000_000_000, true))
	c.ForceFlushAll(spanCh)
	spans := drainSpanCh(spanCh)

	if len(spans) == 0 {
		t.Fatal("no spans emitted")
	}
	sp := spans[0]

	if len(sp.TraceId) != 32 {
		t.Errorf("trace_id should be 32 hex chars, got %d: %q", len(sp.TraceId), sp.TraceId)
	}
	if len(sp.SpanId) != 16 {
		t.Errorf("span_id should be 16 hex chars, got %d: %q", len(sp.SpanId), sp.SpanId)
	}
	for _, ch := range sp.TraceId {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("trace_id has non-hex char %q in %q", ch, sp.TraceId)
		}
	}
}

// TestCorrelatorHTTPExtraction checks HTTP method/path parsing from SSL plaintext.
func TestCorrelatorHTTPExtraction(t *testing.T) {
	cases := []struct {
		raw    string
		method string
		path   string
	}{
		{"GET /api/traces HTTP/1.1\r\nHost: x\r\n\r\n\x00", "GET", "/api/traces"},
		{"POST /ingest HTTP/1.1\r\n\x00", "POST", "/ingest"},
		{"not http at all\x00", "", ""},
		{"DELETE /resource HTTP/1.1\r\n\x00", "DELETE", "/resource"},
	}
	for _, tc := range cases {
		m, p := correlator.ExtractHTTPRequest([]byte(tc.raw))
		if m != tc.method {
			t.Errorf("input=%q: want method=%q got %q", tc.raw, tc.method, m)
		}
		if p != tc.path {
			t.Errorf("input=%q: want path=%q got %q", tc.raw, tc.path, p)
		}
	}
}

// TestCorrelatorRecvTracked checks recv bytes are tracked separately.
func TestCorrelatorRecvTracked(t *testing.T) {
	c := correlator.NewCorrelator(newTestLogger(t))
	spanCh := make(chan *proto.Span, 10)

	c.HandleTCPEventFromInput(correlator.MakeTCPEvent(1, 1, "10.0.0.1", "10.0.0.2", 3000, 80, 1024, 1e9, false))
	c.ForceFlushAll(spanCh)
	spans := drainSpanCh(spanCh)

	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Tags["bytes.received"] != "1024" {
		t.Errorf("expected bytes.received=1024, got %q", spans[0].Tags["bytes.received"])
	}
}
