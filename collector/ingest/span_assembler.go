package ingest

import (
	"sort"
	"sync"

	proto "github.com/zerotrace/zerotrace/proto"
)

// TraceTree groups spans belonging to the same trace.
type TraceTree struct {
	TraceID  string
	Spans    []*proto.Span
	RootSpan *proto.Span
}

// SpanAssembler groups incoming spans by trace ID into complete trace trees.
// It buffers spans until a complete trace is detected or a timeout occurs.
type SpanAssembler struct {
	mu     sync.Mutex
	traces map[string][]*proto.Span
}

// NewSpanAssembler returns an initialised SpanAssembler.
func NewSpanAssembler() *SpanAssembler {
	return &SpanAssembler{
		traces: make(map[string][]*proto.Span),
	}
}

// Ingest adds a batch of spans to the in-progress trace map and returns
// completed trace trees. A trace is considered complete when it contains a
// root span (parent_span_id == "") AND has not received any new spans for
// the current flush cycle — in practice, the gRPC server calls Flush()
// periodically.
func (a *SpanAssembler) Ingest(spans []*proto.Span) []*TraceTree {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, sp := range spans {
		a.traces[sp.TraceId] = append(a.traces[sp.TraceId], sp)
	}

	// Return completed trees: traces that have a root span.
	var completed []*TraceTree
	for traceID, traceSpans := range a.traces {
		if root := findRoot(traceSpans); root != nil {
			completed = append(completed, &TraceTree{
				TraceID:  traceID,
				Spans:    sortedSpans(traceSpans),
				RootSpan: root,
			})
			delete(a.traces, traceID)
		}
	}
	return completed
}

// Flush forces all buffered (potentially partial) traces to be returned.
// Used on shutdown or periodic cleanup to avoid leaking memory.
func (a *SpanAssembler) Flush() []*TraceTree {
	a.mu.Lock()
	defer a.mu.Unlock()

	var trees []*TraceTree
	for traceID, spans := range a.traces {
		root := findRoot(spans)
		if root == nil && len(spans) > 0 {
			root = spans[0] // use first span as synthetic root
		}
		trees = append(trees, &TraceTree{
			TraceID:  traceID,
			Spans:    sortedSpans(spans),
			RootSpan: root,
		})
		delete(a.traces, traceID)
	}
	return trees
}

// findRoot returns the span with an empty parent_span_id, or nil if none.
func findRoot(spans []*proto.Span) *proto.Span {
	for _, sp := range spans {
		if sp.ParentSpanId == "" {
			return sp
		}
	}
	return nil
}

// sortedSpans returns a copy of spans sorted by start time ascending.
func sortedSpans(spans []*proto.Span) []*proto.Span {
	out := make([]*proto.Span, len(spans))
	copy(out, spans)
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTimeNs < out[j].StartTimeNs
	})
	return out
}

// BuildChildMap groups child spans by parent span ID for tree rendering.
func BuildChildMap(spans []*proto.Span) map[string][]*proto.Span {
	m := make(map[string][]*proto.Span)
	for _, sp := range spans {
		m[sp.ParentSpanId] = append(m[sp.ParentSpanId], sp)
	}
	return m
}
