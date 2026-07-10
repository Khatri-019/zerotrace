package store

import (
	"sync"

	proto "github.com/zerotrace/zerotrace/proto"
)

const defaultIndexSize = 1000

// TraceIndex is an in-memory ring buffer of the most recent trace IDs.
// It provides O(1) insertion and O(n) listing of recent trace summaries
// without hitting BadgerDB for the live-tail use case.
type TraceIndex struct {
	mu     sync.RWMutex
	ring   []*TraceSummary
	head   int
	size   int
	cap    int
}

// TraceSummary holds lightweight metadata about one trace for fast listing.
type TraceSummary struct {
	TraceID     string
	RootService string
	RootOp      string
	StartTimeNs int64
	EndTimeNs   int64
	SpanCount   int
}

// NewIndex creates a TraceIndex with a fixed capacity ring buffer.
func NewIndex(cap int) *TraceIndex {
	if cap <= 0 {
		cap = defaultIndexSize
	}
	return &TraceIndex{
		ring: make([]*TraceSummary, cap),
		cap:  cap,
	}
}

// Add inserts a new TraceSummary, overwriting the oldest entry when full.
func (idx *TraceIndex) Add(s *TraceSummary) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.ring[idx.head] = s
	idx.head = (idx.head + 1) % idx.cap
	if idx.size < idx.cap {
		idx.size++
	}
}

// AddFromSpans builds a TraceSummary from a slice of spans and adds it.
func (idx *TraceIndex) AddFromSpans(spans []*proto.Span) {
	if len(spans) == 0 {
		return
	}
	root := spans[0]
	// Find earliest start and latest end
	earliest := root.StartTimeNs
	latest := root.EndTimeNs
	for _, sp := range spans[1:] {
		if sp.StartTimeNs < earliest {
			earliest = sp.StartTimeNs
			root = sp
		}
		if sp.EndTimeNs > latest {
			latest = sp.EndTimeNs
		}
	}
	idx.Add(&TraceSummary{
		TraceID:     root.TraceId,
		RootService: root.ServiceName,
		RootOp:      root.OperationName,
		StartTimeNs: earliest,
		EndTimeNs:   latest,
		SpanCount:   len(spans),
	})
}

// Recent returns the most recent n summaries in newest-first order.
func (idx *TraceIndex) Recent(n int) []*TraceSummary {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if n > idx.size {
		n = idx.size
	}
	result := make([]*TraceSummary, 0, n)
	// Walk backward from head
	for i := 0; i < n; i++ {
		pos := (idx.head - 1 - i + idx.cap) % idx.cap
		if idx.ring[pos] != nil {
			result = append(result, idx.ring[pos])
		}
	}
	return result
}

// Len returns the number of entries currently in the index.
func (idx *TraceIndex) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.size
}
