package exporter

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/zerotrace/zerotrace/proto"
)

// Exporter streams spans to the collector via gRPC.
type Exporter struct {
	addr   string
	conn   *grpc.ClientConn
	client proto.TraceIngestClient
	log    *zap.Logger
}

// New dials the gRPC collector and returns a ready Exporter.
func New(addr string, log *zap.Logger) (*Exporter, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		addr:   addr,
		conn:   conn,
		client: proto.NewTraceIngestClient(conn),
		log:    log,
	}, nil
}

// Close shuts down the underlying gRPC connection.
func (e *Exporter) Close() {
	if e.conn != nil {
		e.conn.Close()
	}
}

// Run reads spans from spanCh, batches them, and sends to the collector.
// It flushes either when batchSize is reached or flushIntervalMS elapses.
// It runs until spanCh is closed.
func (e *Exporter) Run(spanCh <-chan *proto.Span, batchSize, flushIntervalMS int) {
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushIntervalMS <= 0 {
		flushIntervalMS = 100
	}

	ticker := time.NewTicker(time.Duration(flushIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	batch := make([]*proto.Span, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := e.sendBatch(batch); err != nil {
			e.log.Warn("failed to send span batch", zap.Error(err), zap.Int("spans", len(batch)))
		} else {
			e.log.Debug("sent span batch", zap.Int("spans", len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case span, ok := <-spanCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, span)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// sendBatch delivers one batch to the collector, with reconnect logic.
func (e *Exporter) sendBatch(spans []*proto.Span) error {
	// Check connection state and reconnect if needed.
	if state := e.conn.GetState(); state == connectivity.TransientFailure || state == connectivity.Shutdown {
		e.log.Warn("gRPC connection in bad state, reconnecting", zap.String("state", state.String()))
		if err := e.reconnect(); err != nil {
			return err
		}
	}

	req := &proto.SendSpansRequest{
		Batch: &proto.SpanBatch{
			AgentId: "zerotrace-agent",
			Spans:   spans,
		},
	}

	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := e.client.SendSpans(ctx2, req)
	if err != nil {
		return err
	}
	if !resp.Accepted {
		e.log.Warn("collector rejected span batch")
	}
	return nil
}

// reconnect closes the old connection and dials fresh.
func (e *Exporter) reconnect() error {
	if e.conn != nil {
		_ = e.conn.Close()
	}
	conn, err := grpc.NewClient(
		e.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	e.conn = conn
	e.client = proto.NewTraceIngestClient(conn)
	return nil
}
