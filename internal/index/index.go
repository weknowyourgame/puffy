package index

import (
	"fmt"
	"math/rand"

	"github.com/weknowyourgame/puffer/internal/db"
)

// kmeans runs on a flat slab: data holds n vectors of dim floats each,
// vector i is data[i*dim : (i+1)*dim].
// Returns centroids as a flat []float32 of length k*dim, and the cluster
// assignment for each of the n points.
func kmeans(data []float32, dim int, k int, maxIters int) (centroids []float32, assignments []int) {
	n := len(data) / dim

	// Pick k random points from the data as starting centroids.
	centroids = make([]float32, k*dim)
	perm := rand.Perm(n)
	for c := 0; c < k; c++ {
		src := data[perm[c]*dim : (perm[c]+1)*dim]
		copy(centroids[c*dim:(c+1)*dim], src)
	}

	assignments = make([]int, n)

	for iter := 0; iter < maxIters; iter++ {
		// Assign — every point goes to its nearest centroid.
		changed := false
		for i := 0; i < n; i++ {
			point := data[i*dim : (i+1)*dim]

			best := 0
			bestDist, _ := db.L2Squared(point, centroids[0:dim])
			for c := 1; c < k; c++ {
				d, _ := db.L2Squared(point, centroids[c*dim:(c+1)*dim])
				if d < bestDist {
					bestDist = d
					best = c
				}
			}
			if assignments[i] != best {
				changed = true
			}
			assignments[i] = best
		}

		// Update — recompute each centroid as the mean of its group.
		sums := make([]float64, k*dim)
		count := make([]int, k)
		for i := 0; i < n; i++ {
			c := assignments[i]
			point := data[i*dim : (i+1)*dim]
			for j := 0; j < dim; j++ {
				sums[c*dim+j] += float64(point[j])
			}
			count[c]++
		}
		for c := 0; c < k; c++ {
			if count[c] == 0 {
				continue // empty cluster — leave centroid where it was
			}
			for j := 0; j < dim; j++ {
				centroids[c*dim+j] = float32(sums[c*dim+j] / float64(count[c]))
			}
		}

		// Stop once assignments stop changing.
		if !changed {
			fmt.Println("converged after", iter+1, "rounds")
			break
		}
	}

	return centroids, assignments
}
