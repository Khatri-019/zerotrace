package graph_test

import (
	"testing"

	"github.com/zerotrace/zerotrace/collector/graph"
)

// TestRecordEdgeCreatesEntry verifies that RecordEdge creates nodes on first use.
func TestRecordEdgeCreatesEntry(t *testing.T) {
	g := graph.NewDependencyGraph()
	g.RecordEdge("frontend", "backend", 5_000_000, false) // 5ms

	snap := g.Snapshot()
	if len(snap.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(snap.Nodes))
	}
	if len(snap.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(snap.Edges))
	}
}

// TestRecordEdgeCallCount verifies call counting.
func TestRecordEdgeCallCount(t *testing.T) {
	g := graph.NewDependencyGraph()
	for i := 0; i < 10; i++ {
		g.RecordEdge("a", "b", 1_000_000, false)
	}
	snap := g.Snapshot()
	if snap.Edges[0].CallCount != 10 {
		t.Errorf("expected call_count=10, got %d", snap.Edges[0].CallCount)
	}
}

// TestRecordEdgeErrorRate verifies error rate calculation.
func TestRecordEdgeErrorRate(t *testing.T) {
	g := graph.NewDependencyGraph()
	// 1 success, 1 error
	g.RecordEdge("a", "b", 1_000_000, false)
	g.RecordEdge("a", "b", 1_000_000, true)

	snap := g.Snapshot()
	e := snap.Edges[0]
	if e.ErrorRate != 0.5 {
		t.Errorf("expected error_rate=0.5, got %f", e.ErrorRate)
	}
}

// TestRecordEdgeIgnoresSelfLoop ensures self-loops (from == to) are skipped.
func TestRecordEdgeIgnoresSelfLoop(t *testing.T) {
	g := graph.NewDependencyGraph()
	g.RecordEdge("svc", "svc", 1_000_000, false)

	snap := g.Snapshot()
	if len(snap.Edges) != 0 {
		t.Errorf("self-loop should not be recorded, got %d edges", len(snap.Edges))
	}
}

// TestRecordEdgeLatencyHistogram checks that latency is recorded and visible in snapshot.
func TestRecordEdgeLatencyHistogram(t *testing.T) {
	g := graph.NewDependencyGraph()
	// 100ms latency
	g.RecordEdge("a", "b", 100_000_000, false)

	snap := g.Snapshot()
	if snap.Edges[0].P50Ms < 90 || snap.Edges[0].P50Ms > 110 {
		t.Errorf("p50 should be ~100ms, got %f", snap.Edges[0].P50Ms)
	}
}

// TestSnapshotNodeCount checks that nodes are de-duplicated across edges.
func TestSnapshotNodeCount(t *testing.T) {
	g := graph.NewDependencyGraph()
	g.RecordEdge("a", "b", 1e6, false)
	g.RecordEdge("b", "c", 1e6, false)
	g.RecordEdge("a", "c", 1e6, false)

	snap := g.Snapshot()
	// Should have nodes: a, b, c (3 unique)
	if len(snap.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(snap.Nodes))
	}
	if len(snap.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(snap.Edges))
	}
}
