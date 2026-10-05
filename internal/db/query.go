package db

import (
	"container/heap"
	"fmt"
	"sort"
)

/*
 Compute cosine scores of v against
 all the vectors we have
 we return "doc1" : score
*/

type Result struct {
	Key   string  `json:"id"`
	Score float32 `json:"score"`
}

/*
Return K top vectors from similarity search
*/
func (db *DB) Query(v Vector, k int) ([]Result, error) {
	if k < 1 {
		return nil, fmt.Errorf("top k vectors should be a positive number")
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// Pick the distance function once, outside the loop.
	var score func(a, b []float32) (float32, error)
	negate := false
	switch db.metric {
	case Cosine:
		score = CosineSimilarity
	case L2:
		score = L2Squared
		negate = true // higher-is-better holds after negation
	default:
		return nil, fmt.Errorf("unsupported metric: %v", db.metric)
	}

	results := make(ResultHeap, 0, k)
	for key, val := range db.vectors {
		s, err := score(v.Values, val)
		if err != nil {
			continue // matches the previous behavior of ignoring errors; consider returning err instead
		}
		if negate {
			s = -s
		}
		if results.Len() < k {
			heap.Push(&results, Result{key, s})
		} else if s > results[0].Score {
			heap.Pop(&results)
			heap.Push(&results, Result{key, s})
		}
	}

	// Undo the negation so callers see real L2 distances.
	if negate {
		for i := range results {
			results[i].Score = -results[i].Score
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if negate {
			return results[i].Score < results[j].Score // smaller distance first
		}
		return results[i].Score > results[j].Score // larger similarity first
	})

	return results, nil
}
