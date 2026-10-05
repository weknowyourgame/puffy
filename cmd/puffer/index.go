package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/weknowyourgame/puffer/internal/index"
	"github.com/weknowyourgame/puffer/internal/store"
	"github.com/weknowyourgame/puffer/internal/wal"
)

// runIndex reads the whole WAL, builds an index from the final state and writes it.
func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	k := fs.Int("k", 0, "number of clusters (0 = sqrt of the number of vectors)")
	iters := fs.Int("iters", 25, "max k-means rounds")
	fs.Parse(args)

	st := store.NewStore(dataDir)

	// Replay in order: the last write to an id wins, a delete removes it.
	state := make(map[string][]float32)
	var seq uint64
	err := wal.Replay(st, 0, func(s uint64, ops []wal.Operation) {
		for _, op := range ops {
			switch op.Type {
			case wal.Upsert:
				state[op.ID] = op.Values
			case wal.Delete:
				delete(state, op.ID)
			}
		}
		seq = s // Replay goes in order, so the last one is the highest
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(state) == 0 {
		log.Fatal("nothing to index: the WAL has no vectors")
	}

	// Flat slab, sorted by id so the same WAL always gives the same layout.
	keys := make([]string, 0, len(state))
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	dim := len(state[keys[0]])
	data := make([]float32, 0, len(keys)*dim)
	for _, key := range keys {
		if len(state[key]) != dim {
			log.Fatalf("vector %q has %d numbers, expected %d", key, len(state[key]), dim)
		}
		data = append(data, state[key]...)
	}

	clusters := *k
	if clusters == 0 {
		clusters = int(math.Sqrt(float64(len(keys))))
	}
	if clusters < 1 {
		clusters = 1
	}

	idx := index.Build(keys, data, dim, clusters, *iters)
	if err := idx.Write(st, seq); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("indexed %d vectors into %d clusters, covers wal up to %d\n", len(keys), idx.K(), seq)
}
