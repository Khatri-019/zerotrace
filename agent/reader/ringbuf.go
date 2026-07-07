package reader

import (
	"context"
	"go.uber.org/zap"
	"github.com/cilium/ebpf/ringbuf"
)

type RawEvent struct {
	Data []byte
}

func PollRingBuffer(ctx context.Context, reader *ringbuf.Reader, eventCh chan<- RawEvent, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if ringbuf.IsClosed(err) {
				return
			}
			log.Warn("ringbuf read error", zap.Error(err))
			continue
		}

		select {
		case eventCh <- RawEvent{Data: record.RawSample}:
		default:
			log.Warn("ringbuf drops", zap.String("reason", "channel full"))
		}
	}
}
