package topology

import (
	"math"
	"runtime"
	"sync/atomic"
)

// runDBSCAN performs DBSCAN clustering on L2-normalised vectors using cosine
// distance. Labels are returned per-vector: -1 = noise, 1..k = cluster id.
// The zero value is used internally as "unvisited" and never appears in the
// output.
//
// If eps <= 0 a default of 0.3 is used (cosine distance ≈ 70% similarity).
func runDBSCAN(vectors [][]float32, eps float64, minPts int) []int {
	if eps <= 0 {
		eps = 0.3
	}
	n := len(vectors)
	if n == 0 {
		return nil
	}

	neighbors := buildNeighborLists(vectors, eps)

	const unvisited = 0
	const noise = -1

	labels := make([]int, n)
	// labels[i] == 0 means unvisited

	clusterID := 0

	for i := 0; i < n; i++ {
		if labels[i] != unvisited {
			continue
		}
		if len(neighbors[i]) < minPts {
			labels[i] = noise
			continue
		}
		// Start a new cluster.
		clusterID++
		labels[i] = clusterID

		// Seed set (indices to expand).
		seed := make([]int, 0, len(neighbors[i]))
		seed = append(seed, neighbors[i]...)

		for si := 0; si < len(seed); si++ {
			q := seed[si]
			if labels[q] == noise {
				labels[q] = clusterID
				continue
			}
			if labels[q] != unvisited {
				continue
			}
			labels[q] = clusterID
			if len(neighbors[q]) >= minPts {
				seed = append(seed, neighbors[q]...)
			}
		}
	}

	return labels
}

// buildNeighborLists computes for each vector the list of indices within eps
// cosine distance. Work is distributed across runtime.NumCPU() goroutines using
// an atomic counter.
func buildNeighborLists(vectors [][]float32, eps float64) [][]int {
	n := len(vectors)
	if n == 0 {
		return nil
	}
	result := make([][]int, n)

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	var counter int64
	done := make(chan struct{})

	for w := 0; w < numWorkers; w++ {
		go func() {
			for {
				i := int(atomic.AddInt64(&counter, 1)) - 1
				if i >= n {
					break
				}
				list := make([]int, 0, 8)
				for j := 0; j < n; j++ {
					if j == i {
						continue
					}
					if unitCosineDistance(vectors[i], vectors[j]) <= eps {
						list = append(list, j)
					}
				}
				result[i] = list
			}
			done <- struct{}{}
		}()
	}

	for w := 0; w < numWorkers; w++ {
		<-done
	}

	return result
}

// unitCosineDistance returns 1 - dot(a,b), clipped to [0, 2].
// Assumes vectors are already L2-normalised so no division is needed.
func unitCosineDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return 2.0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	dist := 1.0 - dot
	if dist < 0 {
		dist = 0
	}
	if dist > 2 {
		dist = 2
	}
	return dist
}

// l2Normalise normalises each vector in-place. Zero-norm vectors are left
// unchanged.
func l2Normalise(vectors [][]float32) {
	for _, v := range vectors {
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		if norm == 0 {
			continue
		}
		inv := float32(1.0 / math.Sqrt(norm))
		for i := range v {
			v[i] *= inv
		}
	}
}
