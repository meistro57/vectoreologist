package topology

import (
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// pcaDims is the target number of PCA components used before DBSCAN.
const pcaDims = 50

// pcaReduce projects vectors down to nComponents using PCA (covariance matrix
// approach). Only a d×d covariance matrix is ever allocated — never an n×d
// float64 matrix — so memory use is bounded by the vector dimensionality, not
// the dataset size.
//
// If nComponents >= d (the current dimensionality) or nComponents <= 0 the
// input slice is returned unchanged without any allocation.
func pcaReduce(vectors [][]float32, nComponents int) [][]float32 {
	if len(vectors) == 0 {
		return vectors
	}
	d := len(vectors[0])
	if nComponents <= 0 || nComponents >= d {
		return vectors
	}
	n := len(vectors)

	// Step 1: compute column means in float64 to avoid accumulated rounding.
	mean := make([]float64, d)
	for _, v := range vectors {
		for j := 0; j < d; j++ {
			mean[j] += float64(v[j])
		}
	}
	invN := 1.0 / float64(n)
	for j := range mean {
		mean[j] *= invN
	}

	// Step 2: build the d×d covariance matrix in parallel. Each goroutine owns
	// its own flat accumulator so there is no lock contention in the inner loop;
	// we sum them at the end.
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > n {
		numWorkers = n
	}

	// Each worker accumulates a flat d×d upper-triangle + diagonal array.
	size := d * d
	partials := make([][]float64, numWorkers)
	for i := range partials {
		partials[i] = make([]float64, size)
	}

	rowsPerWorker := (n + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		w := w
		start := w * rowsPerWorker
		end := start + rowsPerWorker
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			acc := partials[w]
			for i := start; i < end; i++ {
				v := vectors[i]
				for r := 0; r < d; r++ {
					dr := float64(v[r]) - mean[r]
					for c := r; c < d; c++ {
						dc := float64(v[c]) - mean[c]
						acc[r*d+c] += dr * dc
					}
				}
			}
		}()
	}
	wg.Wait()

	// Sum partial accumulators into the first one.
	total := partials[0]
	for w := 1; w < numWorkers; w++ {
		for i := 0; i < size; i++ {
			total[i] += partials[w][i]
		}
	}

	// Scale by 1/(n-1) and fill the symmetric lower triangle.
	scale := 1.0 / float64(n-1)
	cov := make([]float64, size)
	for r := 0; r < d; r++ {
		for c := r; c < d; c++ {
			val := total[r*d+c] * scale
			cov[r*d+c] = val
			cov[c*d+r] = val
		}
	}

	// Step 3: eigen-decomposition of the symmetric covariance matrix.
	sym := mat.NewSymDense(d, cov)
	var eig mat.EigenSym
	ok := eig.Factorize(sym, true)
	if !ok {
		// Fallback: return vectors unchanged if factorization fails.
		return vectors
	}
	var evecs mat.Dense
	eig.VectorsTo(&evecs)
	// EigenSym returns eigenvectors in ascending eigenvalue order, so the last
	// nComponents columns correspond to the top principal components.

	// Step 4: project each vector. Output is float32 to keep memory tight.
	out := make([][]float32, n)
	for i, v := range vectors {
		row := make([]float32, nComponents)
		for j := 0; j < nComponents; j++ {
			col := d - nComponents + j // column index in evecs (ascending order → last = largest)
			var acc float64
			for k := 0; k < d; k++ {
				acc += (float64(v[k]) - mean[k]) * evecs.At(k, col)
			}
			row[j] = float32(acc)
		}
		out[i] = row
	}
	return out
}
