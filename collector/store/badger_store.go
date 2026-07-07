package store

import (
	"time"
	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
	ttl time.Duration
}

func NewBadgerStore(path string, ttl time.Duration) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable badger logger
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db, ttl: ttl}, nil
}

func (s *BadgerStore) WriteTrace(key string, data []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), data).WithTTL(s.ttl)
		return txn.SetEntry(e)
	})
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}
