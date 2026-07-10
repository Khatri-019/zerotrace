package ingest

import (
	"context"
	"io"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/zerotrace/zerotrace/collector/graph"
	"github.com/zerotrace/zerotrace/collector/store"
	proto "github.com/zerotrace/zerotrace/proto"
)

// SpanSink is the interface the gRPC server uses to deliver assembled traces.
// Both BadgerStore and the WebSocket Hub satisfy this via composition in main.
type SpanSink interface {
	// OnTrace is called with each completed trace tree.
	OnTrace(tree *TraceTree)
}

// GRPCServer implements the TraceIngest gRPC service.
type GRPCServer struct {
	proto.UnimplementedTraceIngestServer
	log       *zap.Logger
	assembler *SpanAssembler
	store     *store.BadgerStore
	index     *store.TraceIndex
	graph     *graph.DependencyGraph
	// onTrace is a callback for each completed trace (used to broadcast to WebSocket clients)
	onTrace func(spans []*proto.Span)
}

// NewGRPCServer creates a GRPCServer wired to all backend components.
func NewGRPCServer(
	log *zap.Logger,
	badger *store.BadgerStore,
	index *store.TraceIndex,
	graph *graph.DependencyGraph,
	onTrace func(spans []*proto.Span),
) *GRPCServer {
	return &GRPCServer{
		log:       log,
		assembler: NewSpanAssembler(),
		store:     badger,
		index:     index,
		graph:     graph,
		onTrace:   onTrace,
	}
}

// SendSpans handles the unary RPC.
func (s *GRPCServer) SendSpans(_ context.Context, req *proto.SendSpansRequest) (*proto.SendSpansResponse, error) {
	if req.Batch == nil {
		return &proto.SendSpansResponse{Accepted: false}, nil
	}
	s.ingestBatch(req.Batch)
	return &proto.SendSpansResponse{Accepted: true}, nil
}

// StreamSpans handles the bidirectional streaming RPC.
func (s *GRPCServer) StreamSpans(stream proto.TraceIngest_StreamSpansServer) error {
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.ingestBatch(batch)
		if err := stream.Send(&proto.Ack{SpansAccepted: uint64(len(batch.Spans))}); err != nil {
			return err
		}
	}
}

// ingestBatch processes one SpanBatch: assembles traces, persists them,
// updates the dependency graph, and notifies the WebSocket hub.
func (s *GRPCServer) ingestBatch(batch *proto.SpanBatch) {
	if len(batch.Spans) == 0 {
		return
	}

	trees := s.assembler.Ingest(batch.Spans)
	for _, tree := range trees {
		s.processTrees(tree)
	}

	// Even if no tree is complete, still log incoming span count
	s.log.Info("ingested spans",
		zap.String("agent", batch.AgentId),
		zap.String("host", batch.Host),
		zap.Int("spans", len(batch.Spans)),
		zap.Int("completed_traces", len(trees)),
	)
}

// processTrees persists one completed trace, updates graph, and broadcasts.
func (s *GRPCServer) processTrees(tree *TraceTree) {
	// 1. Persist to BadgerDB
	if err := s.store.WriteTrace(tree.TraceID, tree.Spans); err != nil {
		s.log.Error("store write failed", zap.String("trace_id", tree.TraceID), zap.Error(err))
	}

	// 2. Update in-memory index
	s.index.AddFromSpans(tree.Spans)

	// 3. Update dependency graph — record edges between sequential spans
	s.updateGraph(tree.Spans)

	// 4. Broadcast to WebSocket clients
	if s.onTrace != nil {
		s.onTrace(tree.Spans)
	}
}

// updateGraph records call edges from parent → child span services.
func (s *GRPCServer) updateGraph(spans []*proto.Span) {
	// Build a spanID → span lookup
	byID := make(map[string]*proto.Span, len(spans))
	for _, sp := range spans {
		byID[sp.SpanId] = sp
	}

	for _, child := range spans {
		if child.ParentSpanId == "" {
			continue
		}
		parent, ok := byID[child.ParentSpanId]
		if !ok {
			continue
		}
		latencyNs := child.EndTimeNs - child.StartTimeNs
		isError := child.Tags["http.status_code"] >= "500"
		s.graph.RecordEdge(parent.ServiceName, child.ServiceName, latencyNs, isError)
	}
}

// Start binds to address and serves the gRPC server in a background goroutine.
func Start(address string, srv *GRPCServer, log *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16 * 1024 * 1024), // 16 MiB
	)
	proto.RegisterTraceIngestServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server exited", zap.Error(err))
		}
	}()
	log.Info("gRPC server listening", zap.String("address", address))
	return grpcServer, nil
}
