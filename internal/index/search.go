package index

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"

	"github.com/weknowyourgame/puffer/internal/db"
)

var ErrNoIndex = errors.New("no index found")

/*
Index groups vectors into clusters (k-means). A query only looks inside
the few clusters whose centroid is closest to it.

The vectors of a cluster are fetched through loadCluster, so the same
search works on clusters held in memory (Build) and on clusters read
from a file one at a time (Open).
*/
type Index struct {
	centroids []float32 // k*dim, centroid c is centroids[c*dim : (c+1)*dim]
	dim       int
	k         int
	seq       uint64 // the index covers every WAL entry up to and including this one

	// in memory only (nil when opened from a file)
	clusterIDs  [][]string
	clusterData [][]float32

	loadCluster func(c int) (ids []string, data []float32, err error)
}

// Build clusters n vectors. keys[i] is the id of data[i*dim : (i+1)*dim].
func Build(keys []string, data []float32, dim int, k int, maxIters int) *Index {
	n := len(data) / dim
	if k > n {
		k = n
	}

	centroids, assignments := kmeans(data, dim, k, maxIters)

	idx := &Index{
		centroids:   centroids,
		dim:         dim,
		k:           k,
		clusterIDs:  make([][]string, k),
		clusterData: make([][]float32, k),
	}

	// Put each cluster's vectors next to each other.
	for i, c := range assignments {
		idx.clusterIDs[c] = append(idx.clusterIDs[c], keys[i])
		idx.clusterData[c] = append(idx.clusterData[c], data[i*dim:(i+1)*dim]...)
	}

	idx.loadCluster = func(c int) ([]string, []float32, error) {
		return idx.clusterIDs[c], idx.clusterData[c], nil
	}
	return idx
}

func (idx *Index) Seq() uint64 { return idx.seq }
func (idx *Index) K() int      { return idx.k }

/*
Search returns the topK closest vectors (smallest L2 distance first),
only looking inside the nprobe clusters nearest to the query.
skip lets the caller ignore ids (nil skips nothing).
*/
func (idx *Index) Search(query []float32, topK int, nprobe int, skip func(id string) bool) ([]db.Result, error) {
	if len(query) != idx.dim {
		return nil, fmt.Errorf("query has %d numbers, index expects %d", len(query), idx.dim)
	}
	if topK < 1 {
		return nil, fmt.Errorf("top k vectors should be a positive number")
	}
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > idx.k {
		nprobe = idx.k
	}

	// Distance from the query to every centroid, nearest first.
	type centroidDist struct {
		c    int
		dist float32
	}
	order := make([]centroidDist, idx.k)
	for c := 0; c < idx.k; c++ {
		d, _ := db.L2Squared(query, idx.centroids[c*idx.dim:(c+1)*idx.dim])
		order[c] = centroidDist{c, d}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].dist < order[j].dist })

	// Same trick as db.Query: negate so higher is better, and the min-heap root is the worst kept.
	results := make(db.ResultHeap, 0, topK)
	for _, p := range order[:nprobe] {
		ids, vecs, err := idx.loadCluster(p.c)
		if err != nil {
			return nil, err
		}
		for i, id := range ids {
			if skip != nil && skip(id) {
				continue
			}
			d, _ := db.L2Squared(query, vecs[i*idx.dim:(i+1)*idx.dim])
			s := -d
			if results.Len() < topK {
				heap.Push(&results, db.Result{Key: id, Score: s})
			} else if s > results[0].Score {
				heap.Pop(&results)
				heap.Push(&results, db.Result{Key: id, Score: s})
			}
		}
	}

	for i := range results {
		results[i].Score = -results[i].Score
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score < results[j].Score })
	return results, nil
}
