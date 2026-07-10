package reader

import (
	"context"
	"errors"

	"github.com/cilium/ebpf/ringbuf"
	"go.uber.org/zap"
)

// PollRingBuffer reads raw bytes from the eBPF ring buffer, parses them into
// typed RawEvents, and sends them to eventCh. It runs until ctx is cancelled
// or the ring buffer is closed.
func PollRingBuffer(ctx context.Context, reader *ringbuf.Reader, eventCh chan<- RawEvent, log *zap.Logger) {
	for {
		// Check context first to avoid blocking on Read after cancellation.
		select {
		case <-ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			log.Warn("ringbuf read error", zap.Error(err))
			continue
		}

		ev, err := ParseEvent(record.RawSample)
		if err != nil {
			log.Debug("event parse error", zap.Error(err), zap.Int("bytes", len(record.RawSample)))
			continue
		}

		select {
		case eventCh <- *ev:
		default:
			log.Warn("event channel full, dropping event", zap.Uint8("kind", ev.Kind))
		}
	}
}
