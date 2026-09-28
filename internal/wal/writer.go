package wal

// N writer go routines (has return error) -> 1 shared chan request -> 1 flusher go routine -> Store.create

import (
	"fmt"
	"time"

	"github.com/weknowyourgame/puffer/internal/db"
	"github.com/weknowyourgame/puffer/internal/store"
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

	w.seq++
	err := w.write(ops)

	for _, req := range batch {
		req.reply <- err
	}
}

func (w *Writer) write(ops []Operation) error {
	data, err := Encode(w.seq, ops)
	if err != nil {
		return err
	}
	return w.store.Create(fmt.Sprintf("wal/%020d.bin", w.seq), data)
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
