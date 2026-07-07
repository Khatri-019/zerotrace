package graph

import (
	"github.com/HdrHistogram/hdrhistogram-go"
	"sync"
)

type EdgeStats struct {
	CallCount  uint64
	ErrorCount uint64
	Latency    *hdrhistogram.Histogram
}

type DependencyGraph struct {
	sync.RWMutex
	Edges map[string]map[string]*EdgeStats
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Edges: make(map[string]map[string]*EdgeStats),
	}
}
