package db

import (
	"fmt"
	"sync"
)

/*
Vector struct stores in a kv pair
along with a string key (hash later)
and the dimensions it will be stored in
*/
type DB struct {
	vectors map[string][]float32
	dim     int
	mu      sync.RWMutex
	metric  Metric
}

type Dimension int

type Metric int

const (
	Dim128 Dimension = 128
	Dim256 Dimension = 256
	Dim512 Dimension = 512
)

const (
	Cosine Metric = iota
	L2
)

type Vector struct {
	Key    string
	Values []float32
}

func New(dim Dimension, m Metric) *DB {
	return &DB{
		vectors: make(map[string][]float32),
		dim:     int(dim),
	}
}

func (db *DB) Upsert(v Vector) error {
	if len(v.Values) != db.dim {
		return fmt.Errorf("dimension mismatch")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	db.vectors[v.Key] = v.Values
	return nil
}

func (db *DB) Delete(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.vectors, key)
}

func (db *DB) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	l := len(db.vectors)
	return l
}

func (db *DB) Dim() int {
	return db.dim
}
