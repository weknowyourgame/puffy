package wal

// N writer go routines (has return error) ->
// 1 shared chan request -> 1 flusher go routine -> Store.create

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/weknowyourgame/puffy/internal/db"
	"github.com/weknowyourgame/puffy/internal/store"
)

const (
	maxBatch = 100
	interval = 5 * time.Millisecond
)

type Request struct {
	ops   []Operation
	reply chan error
}

type Writer struct {
	reqs  chan Request
	store store.Store
	db    *db.DB
	seq   uint64
}

func NewWriter(s store.Store, d *db.DB) (*Writer, error) {
	keys, err := s.List("wal/")
	if err != nil {
		return nil, err
	}

	// recover the highest existing sequence number
	var seq uint64
	for _, key := range keys {
		name := strings.TrimSuffix(strings.TrimPrefix(key, "wal/"), ".bin")
		n, err := strconv.ParseUint(name, 10, 64)
		if err != nil {
			return nil, err
		}
		if n > seq {
			seq = n
		}
	}

	w := &Writer{
		reqs:  make(chan Request, maxBatch),
		store: s,
		db:    d,
		seq:   seq,
	}
	go w.Run(maxBatch, interval)
	return w, nil
}

// make a reply channel send Request{ops, reply} into w.reqs return the value received from reply
func (w *Writer) Write(ops []Operation) error {
	reply := make(chan error, 1)
	w.reqs <- Request{ops: ops, reply: reply}
	return <-reply
}

func (w *Writer) flush(batch []Request) {
	var ops []Operation
	for _, req := range batch {
		ops = append(ops, req.ops...)
	}

	err := w.write(ops)

	for _, req := range batch {
		req.reply <- err
	}
}

func (w *Writer) write(ops []Operation) error {
	next := w.seq + 1

	// file first: durable on disk
	data, err := Encode(next, ops)
	if err != nil {
		return err
	}
	if err := w.store.Create(fmt.Sprintf("wal/%020d.bin", next), data); err != nil {
		return err
	}
	w.seq = next // only advance once Create succeeded, so no gaps

	// then memory: visible to queries
	for _, op := range ops {
		switch op.Type {
		case Upsert:
			w.db.Upsert(db.Vector{Key: op.ID, Values: op.Values})
		case Delete:
			w.db.Delete(op.ID)
		}
	}
	return nil
}

func (w *Writer) Run(maxBatch int, interval time.Duration) {
	ticker := time.NewTicker(interval)
	var batch []Request
	for {
		select {
		case req := <-w.reqs:
			batch = append(batch, req)
			if len(batch) >= maxBatch {
				w.flush(batch)
				batch = nil
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = nil
			}
		}
	}
}
