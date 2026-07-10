package store_test

import (
	"os"
	"testing"
	"time"

	"github.com/zerotrace/zerotrace/collector/store"
	proto "github.com/zerotrace/zerotrace/proto"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "zerotrace-badger-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestBadgerWriteAndReadTrace(t *testing.T) {
	s, err := store.NewBadgerStore(tempDir(t), time.Hour)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}
	defer s.Close()

	spans := []*proto.Span{
		{TraceId: "trace1", SpanId: "span1", ServiceName: "svc-a", OperationName: "GET /api", StartTimeNs: 1000, EndTimeNs: 2000},
		{TraceId: "trace1", SpanId: "span2", ServiceName: "svc-b", OperationName: "DB query", StartTimeNs: 1100, EndTimeNs: 1900, ParentSpanId: "span1"},
	}

	if err := s.WriteTrace("trace1", spans); err != nil {
		t.Fatalf("WriteTrace: %v", err)
	}

	got, err := s.GetTrace("trace1")
	if err != nil {
		t.Fatalf("GetTrace: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 spans, got %d", len(got))
	}
}

func TestBadgerListTraceIDs(t *testing.T) {
	s, err := store.NewBadgerStore(tempDir(t), time.Hour)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}
	defer s.Close()

	for i := 0; i < 5; i++ {
		id := "trace" + string(rune('0'+i))
		spans := []*proto.Span{{TraceId: id, SpanId: "s1", ServiceName: "svc", StartTimeNs: int64(i)}}
		_ = s.WriteTrace(id, spans)
	}

	ids, err := s.ListTraceIDs(3, 0)
	if err != nil {
		t.Fatalf("ListTraceIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 results with limit=3, got %d", len(ids))
	}

	all, _ := s.ListTraceIDs(100, 0)
	if len(all) != 5 {
		t.Errorf("expected 5 total, got %d", len(all))
	}
}

func TestBadgerListServices(t *testing.T) {
	s, err := store.NewBadgerStore(tempDir(t), time.Hour)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}
	defer s.Close()

	spans := []*proto.Span{
		{TraceId: "t1", SpanId: "s1", ServiceName: "frontend"},
		{TraceId: "t1", SpanId: "s2", ServiceName: "backend"},
		{TraceId: "t2", SpanId: "s3", ServiceName: "frontend"}, // duplicate
	}
	_ = s.WriteTrace("t1", spans[:2])
	_ = s.WriteTrace("t2", spans[2:])

	svcs, err := s.ListServices()
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	svcSet := make(map[string]bool)
	for _, svc := range svcs {
		svcSet[svc] = true
	}
	if !svcSet["frontend"] || !svcSet["backend"] {
		t.Errorf("expected frontend and backend in services, got %v", svcs)
	}
	if len(svcs) != 2 {
		t.Errorf("expected 2 unique services, got %d", len(svcs))
	}
}

func TestBadgerGetMissingTrace(t *testing.T) {
	s, err := store.NewBadgerStore(tempDir(t), time.Hour)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}
	defer s.Close()

	got, err := s.GetTrace("nonexistent")
	if err != nil {
		t.Fatalf("GetTrace should not error for missing trace: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d spans", len(got))
	}
}
