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
func (db *DB) Query(v Vector, k int) ([]Result, []Result, error) {
	if k < 1 {
		return nil, nil, fmt.Errorf("top k vectors should be a positive number")
	}
	resultsSim := make(ResultHeap, 0, k)
	resultsL2 := make(ResultHeap, 0, k)

	db.mu.RLock()
	defer db.mu.RUnlock()
	for key, val := range db.vectors {
		similarity, _ := CosineSimilarity(v.Values, val)
		if resultsSim.Len() < k {
			heap.Push(&resultsSim, Result{key, similarity})
		} else if similarity > resultsSim[0].Score {
			heap.Pop(&resultsSim)
			heap.Push(&resultsSim, Result{key, similarity})
		}

		L2Distance, _ := L2Squared(v.Values, val)
		negL2 := -L2Distance // min-heap over negated distance = max-heap over real distance
		if resultsL2.Len() < k {
			heap.Push(&resultsL2, Result{key, negL2})
		} else if negL2 > resultsL2[0].Score {
			// equivalent to: L2Distance < current worst kept distance
			heap.Pop(&resultsL2)
			heap.Push(&resultsL2, Result{key, negL2})
		}
	}

	// Fix: add proper type to heap.go later
	// undo the negation before sorting/returning
	for i := range resultsL2 {
		resultsL2[i].Score = -resultsL2[i].Score
	}

	sort.Slice(resultsSim, func(i, j int) bool {
		return resultsSim[i].Score > resultsSim[j].Score
	})

	sort.Slice(resultsL2, func(i, j int) bool {
		return resultsL2[i].Score < resultsL2[j].Score
	})

	return resultsSim, resultsL2, nil
}
