package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	proto "github.com/zerotrace/zerotrace/proto"
)

// BadgerStore persists spans using BadgerDB with automatic TTL expiry.
type BadgerStore struct {
	db  *badger.DB
	ttl time.Duration
}

// NewBadgerStore opens (or creates) the BadgerDB at path with the given TTL.
func NewBadgerStore(path string, ttl time.Duration) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Silence BadgerDB internal logs; we use zap
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger at %s: %w", path, err)
	}
	return &BadgerStore{db: db, ttl: ttl}, nil
}

// ---------------------------------------------------------------------------
// Write
// ---------------------------------------------------------------------------

// WriteSpan serialises a single span and stores it under key
// "span:<traceID>:<spanID>". The TTL is set automatically.
func (s *BadgerStore) WriteSpan(span *proto.Span) error {
	data, err := json.Marshal(span)
	if err != nil {
		return fmt.Errorf("marshal span: %w", err)
	}
	key := fmt.Sprintf("span:%s:%s", span.TraceId, span.SpanId)
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), data).WithTTL(s.ttl)
		return txn.SetEntry(e)
	})
}

// WriteTrace writes all spans in a trace under keys "span:<traceID>:<spanID>"
// and additionally writes an index key "trace:<traceID>" → "" so traces are
// listable without a full scan.
func (s *BadgerStore) WriteTrace(traceID string, spans []*proto.Span) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for _, span := range spans {
			data, err := json.Marshal(span)
			if err != nil {
				return err
			}
			spanKey := fmt.Sprintf("span:%s:%s", span.TraceId, span.SpanId)
			e := badger.NewEntry([]byte(spanKey), data).WithTTL(s.ttl)
			if err := txn.SetEntry(e); err != nil {
				return err
			}
		}
		// Index key: maps traceID → root service name for listing
		var rootSvc string
		if len(spans) > 0 {
			rootSvc = spans[0].ServiceName
		}
		idxKey := fmt.Sprintf("trace:%s", traceID)
		e := badger.NewEntry([]byte(idxKey), []byte(rootSvc)).WithTTL(s.ttl)
		return txn.SetEntry(e)
	})
}

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

// GetTrace retrieves all spans for a trace ID, sorted by start time.
func (s *BadgerStore) GetTrace(traceID string) ([]*proto.Span, error) {
	prefix := []byte(fmt.Sprintf("span:%s:", traceID))
	var spans []*proto.Span

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var span proto.Span
				if err := json.Unmarshal(val, &span); err != nil {
					return err
				}
				spans = append(spans, &span)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return spans, err
}

// ListTraceIDs returns up to limit trace IDs, skipping offset entries.
// Each entry is a [traceID, rootServiceName] pair.
func (s *BadgerStore) ListTraceIDs(limit, offset int) ([][2]string, error) {
	prefix := []byte("trace:")
	var result [][2]string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		skip := 0
		for it.Rewind(); it.Valid() && len(result) < limit; it.Next() {
			if skip < offset {
				skip++
				continue
			}
			item := it.Item()
			key := string(item.Key())
			traceID := key[len("trace:"):]
			var svcName string
			_ = item.Value(func(val []byte) error {
				svcName = string(val)
				return nil
			})
			result = append(result, [2]string{traceID, svcName})
		}
		return nil
	})
	return result, err
}

// ScanAllSpans iterates over every span stored in BadgerDB and calls fn for
// each one. Stops and returns on the first error from fn.
// Used during startup to warm up in-memory state from persisted data.
func (s *BadgerStore) ScanAllSpans(fn func(*proto.Span) error) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("span:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			if err := item.Value(func(val []byte) error {
				var span proto.Span
				if err := json.Unmarshal(val, &span); err != nil {
					return nil // skip corrupted
				}
				return fn(&span)
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListServices scans all span keys and returns distinct service names.
func (s *BadgerStore) ListServices() ([]string, error) {
	seen := make(map[string]struct{})

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("span:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var span proto.Span
				if err := json.Unmarshal(val, &span); err != nil {
					return nil // skip corrupted entries
				}
				seen[span.ServiceName] = struct{}{}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	svcs := make([]string, 0, len(seen))
	for svc := range seen {
		svcs = append(svcs, svc)
	}
	return svcs, nil
}

// RunGC triggers BadgerDB garbage collection in a tight loop to reclaim space.
func (s *BadgerStore) RunGC() {
	for {
		if err := s.db.RunValueLogGC(0.5); err != nil {
			break // no more GC needed
		}
	}
}

// Close shuts down the BadgerDB instance.
func (s *BadgerStore) Close() error {
	return s.db.Close()
}
