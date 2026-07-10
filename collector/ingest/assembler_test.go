package ingest_test

import (
	"testing"

	"github.com/zerotrace/zerotrace/collector/ingest"
	proto "github.com/zerotrace/zerotrace/proto"
)

func makeSpan(traceID, spanID, parentID, service string, startNs, endNs int64) *proto.Span {
	return &proto.Span{
		TraceId:       traceID,
		SpanId:        spanID,
		ParentSpanId:  parentID,
		ServiceName:   service,
		OperationName: "test-op",
		StartTimeNs:   startNs,
		EndTimeNs:     endNs,
	}
}

// TestAssemblerSingleSpanCompletes tests that a single root span (no parent)
// is immediately assembled into a complete trace.
func TestAssemblerSingleSpanCompletes(t *testing.T) {
	a := ingest.NewSpanAssembler()
	spans := []*proto.Span{
		makeSpan("trace1", "span1", "", "svc-a", 1000, 2000),
	}
	trees := a.Ingest(spans)
	if len(trees) != 1 {
		t.Fatalf("expected 1 completed tree, got %d", len(trees))
	}
	if trees[0].TraceID != "trace1" {
		t.Errorf("wrong trace ID: %q", trees[0].TraceID)
	}
	if trees[0].RootSpan == nil {
		t.Error("root span should not be nil")
	}
}

// TestAssemblerMultiSpanTree tests that child spans without a root stay buffered
// until the root arrives, then all are assembled together.
func TestAssemblerMultiSpanTree(t *testing.T) {
	a := ingest.NewSpanAssembler()

	// Ingest child first
	child := []*proto.Span{
		makeSpan("trace2", "span2", "span1", "svc-b", 1100, 1900),
	}
	trees := a.Ingest(child)
	if len(trees) != 0 {
		t.Errorf("expected 0 trees before root arrives, got %d", len(trees))
	}

	// Now ingest root
	root := []*proto.Span{
		makeSpan("trace2", "span1", "", "svc-a", 1000, 2000),
	}
	trees = a.Ingest(root)
	if len(trees) != 1 {
		t.Fatalf("expected 1 tree after root arrives, got %d", len(trees))
	}
	if len(trees[0].Spans) != 2 {
		t.Errorf("expected 2 spans in tree, got %d", len(trees[0].Spans))
	}
}

// TestAssemblerSpansSortedByStartTime checks that assembled spans are in
// chronological order regardless of ingest order.
func TestAssemblerSpansSortedByStartTime(t *testing.T) {
	a := ingest.NewSpanAssembler()
	spans := []*proto.Span{
		makeSpan("trace3", "s3", "s1", "svc", 3000, 4000),
		makeSpan("trace3", "s1", "",   "svc", 1000, 5000),
		makeSpan("trace3", "s2", "s1", "svc", 2000, 3000),
	}
	trees := a.Ingest(spans)
	if len(trees) != 1 {
		t.Fatalf("expected 1 tree, got %d", len(trees))
	}
	for i := 1; i < len(trees[0].Spans); i++ {
		if trees[0].Spans[i].StartTimeNs < trees[0].Spans[i-1].StartTimeNs {
			t.Error("spans not sorted by start time")
		}
	}
}

// TestAssemblerFlushReturnsPartials ensures Flush() drains buffered partial traces.
func TestAssemblerFlushReturnsPartials(t *testing.T) {
	a := ingest.NewSpanAssembler()
	// Ingest two orphaned child spans (no root → never complete)
	a.Ingest([]*proto.Span{
		makeSpan("traceX", "s1", "missing-parent", "svc", 0, 1),
	})
	trees := a.Flush()
	if len(trees) != 1 {
		t.Errorf("Flush should return 1 partial trace, got %d", len(trees))
	}
}

// TestBuildChildMap verifies the child map construction.
func TestBuildChildMap(t *testing.T) {
	spans := []*proto.Span{
		{SpanId: "root", ParentSpanId: ""},
		{SpanId: "child1", ParentSpanId: "root"},
		{SpanId: "child2", ParentSpanId: "root"},
	}
	m := ingest.BuildChildMap(spans)
	if len(m["root"]) != 2 {
		t.Errorf("expected 2 children of root, got %d", len(m["root"]))
	}
}
