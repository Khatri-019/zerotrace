package exporter

import (
	"context"
	"go.uber.org/zap"
	proto "github.com/zerotrace/zerotrace/proto"
)

type Exporter struct {
	log *zap.Logger
}

func New(addr string, log *zap.Logger) (*Exporter, error) {
	return &Exporter{log: log}, nil
}

func (e *Exporter) Close() {}

func (e *Exporter) Run(ctx context.Context, spanCh <-chan *proto.Span, batchSize, flushIntervalMS int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-spanCh:
			// Add to batch and flush based on timer/size
		}
	}
}
