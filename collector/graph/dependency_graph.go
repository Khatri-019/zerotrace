package graph

import (
	"sync"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// EdgeStats holds aggregated statistics for one directed service call.
type EdgeStats struct {
	CallCount  uint64
	ErrorCount uint64
	// Latency histogram tracks request latency in microseconds.
	// Range: 1µs → 60s, 3 significant figures.
	Latency *hdrhistogram.Histogram
}

// newEdgeStats creates an EdgeStats with an initialised histogram.
func newEdgeStats() *EdgeStats {
	return &EdgeStats{
		Latency: hdrhistogram.New(1, 60_000_000, 3),
	}
}

// DependencyGraph tracks call relationships between services.
// Edges[from][to] = EdgeStats for calls from service "from" to service "to".
type DependencyGraph struct {
	mu    sync.RWMutex
	Edges map[string]map[string]*EdgeStats
}

// NewDependencyGraph creates a new empty DependencyGraph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Edges: make(map[string]map[string]*EdgeStats),
	}
}

// RecordEdge records one call from service `from` to service `to` with the
// given latency in nanoseconds. isError should be true for 5xx or failed calls.
func (g *DependencyGraph) RecordEdge(from, to string, latencyNs int64, isError bool) {
	if from == "" || to == "" || from == to {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Edges[from] == nil {
		g.Edges[from] = make(map[string]*EdgeStats)
	}
	if g.Edges[from][to] == nil {
		g.Edges[from][to] = newEdgeStats()
	}

	e := g.Edges[from][to]
	e.CallCount++
	if isError {
		e.ErrorCount++
	}
	// Record latency in microseconds (histogram range is µs)
	latencyUs := latencyNs / 1000
	if latencyUs < 1 {
		latencyUs = 1
	}
	_ = e.Latency.RecordValue(latencyUs)
}

// ---------------------------------------------------------------------------
// Snapshot types for JSON serialisation
// ---------------------------------------------------------------------------

// GraphNode represents a service node in the serialised graph.
type GraphNode struct {
	ID       string  `json:"id"`
	Group    int     `json:"group"`
	P50Ms    float64 `json:"p50_ms"`
	P99Ms    float64 `json:"p99_ms"`
	ErrorRate float64 `json:"error_rate"`
}

// GraphEdge represents a directed edge in the serialised graph.
type GraphEdge struct {
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	CallCount uint64  `json:"call_count"`
	ErrorRate float64 `json:"error_rate"`
	P50Ms     float64 `json:"p50_ms"`
}

// GraphSnapshot is the JSON-serialisable snapshot of the dependency graph.
type GraphSnapshot struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// Snapshot produces a JSON-serialisable snapshot of the dependency graph.
func (g *DependencyGraph) Snapshot() GraphSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeSet := make(map[string]bool)
	var edges []GraphEdge

	for from, targets := range g.Edges {
		nodeSet[from] = true
		for to, stats := range targets {
			nodeSet[to] = true
			errRate := 0.0
			if stats.CallCount > 0 {
				errRate = float64(stats.ErrorCount) / float64(stats.CallCount)
			}
			p50 := float64(stats.Latency.ValueAtQuantile(50)) / 1000.0
			edges = append(edges, GraphEdge{
				Source:    from,
				Target:    to,
				CallCount: stats.CallCount,
				ErrorRate: errRate,
				P50Ms:     p50,
			})
		}
	}

	nodes := make([]GraphNode, 0, len(nodeSet))
	groupIdx := 1
	groupMap := make(map[string]int)
	for svc := range nodeSet {
		groupMap[svc] = groupIdx
		groupIdx++
	}

	for svc := range nodeSet {
		// Aggregate latency for this node across all outbound edges
		var p50, p99 float64
		var totalCalls, totalErrors uint64
		if targets, ok := g.Edges[svc]; ok {
			merged := hdrhistogram.New(1, 60_000_000, 3)
			for _, stats := range targets {
				merged.Merge(stats.Latency)
				totalCalls += stats.CallCount
				totalErrors += stats.ErrorCount
			}
			p50 = float64(merged.ValueAtQuantile(50)) / 1000.0
			p99 = float64(merged.ValueAtQuantile(99)) / 1000.0
		}
		errRate := 0.0
		if totalCalls > 0 {
			errRate = float64(totalErrors) / float64(totalCalls)
		}
		nodes = append(nodes, GraphNode{
			ID:       svc,
			Group:    groupMap[svc],
			P50Ms:    p50,
			P99Ms:    p99,
			ErrorRate: errRate,
		})
	}

	return GraphSnapshot{
		Nodes: nodes,
		Edges: func() []GraphEdge {
			if edges == nil {
				return []GraphEdge{}
			}
			return edges
		}(),
	}
}
