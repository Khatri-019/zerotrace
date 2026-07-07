package correlator

import (
	"context"
	"go.uber.org/zap"
	"github.com/zerotrace/zerotrace/agent/reader"
	proto "github.com/zerotrace/zerotrace/proto"
)

func Run(ctx context.Context, eventCh <-chan reader.RawEvent, spanCh chan<- *proto.Span, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-eventCh:
			// Parse event and correlate
		}
	}
}
