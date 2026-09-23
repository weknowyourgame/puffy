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
	Key   string
	Score float32
}

/*
Return K top vectors from similarity search
*/
func (db *DB) Query(v Vector, k int) ([]Result, error) {
	if k < 1 {
		return nil, fmt.Errorf("top k vectors should be a positive number")
	}
	results := make(ResultHeap, 0, k)

	for key, val := range db.vectors {
		similarity, _ := CosineSimilarity(v.Values, val)
		if results.Len() < k {
			// if the len of heap is not k push
			heap.Push(&results, Result{key, similarity})
		} else if similarity > results[0].Score {
			// pop and push if we get a greater score
			heap.Pop(&results)
			heap.Push(&results, Result{key, similarity})
		}
	}

	// Sort the scores worst at index 0
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}
