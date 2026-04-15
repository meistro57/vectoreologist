#!/usr/bin/env python3
"""
Vectoreologist clustering script.
Reads a JSON input file (path given as argv[1]), writes cluster JSON to stdout.

Input JSON:
  {
    "vectors":  [[float, ...], ...],
    "metadata": [{"id": "123", "source": "...", "layer": "...", "run_id": "..."}, ...],
    "params":   {"n_neighbors": 15, "min_dist": 0.1}
  }

Output JSON:
  {
    "clusters":      [{id, label, vector_ids, centroid, density, size, coherence}, ...],
    "noise_count":   int,
    "total_vectors": int
  }
"""
import json
import sys
import warnings

warnings.filterwarnings("ignore")

import numpy as np


def main():
    if len(sys.argv) < 2:
        print("usage: cluster.py <input.json>", file=sys.stderr)
        sys.exit(1)

    try:
        import umap
        import hdbscan
    except ImportError as e:
        print(f"missing dependency: {e}", file=sys.stderr)
        print("install with: pip install umap-learn hdbscan numpy", file=sys.stderr)
        sys.exit(2)

    with open(sys.argv[1]) as f:
        data = json.load(f)

    vectors  = np.array(data["vectors"], dtype=np.float32)
    metadata = data["metadata"]
    params   = data.get("params", {})

    n_neighbors       = int(params.get("n_neighbors", 15))
    min_dist          = float(params.get("min_dist", 0.1))
    min_cluster_size  = max(5, len(vectors) // 100)

    # UMAP: reduce to 2D for clustering
    reducer   = umap.UMAP(
        n_neighbors=n_neighbors,
        min_dist=min_dist,
        n_components=2,
        random_state=42,
        verbose=False,
    )
    embedding = reducer.fit_transform(vectors)

    # HDBSCAN: density-based clustering on the 2D embedding
    clusterer = hdbscan.HDBSCAN(min_cluster_size=min_cluster_size, prediction_data=True)
    labels    = clusterer.fit_predict(embedding)

    unique_labels = sorted(set(labels) - {-1})
    clusters = []

    for label in unique_labels:
        mask    = labels == label
        indices = np.where(mask)[0]
        vecs    = vectors[mask]

        centroid    = vecs.mean(axis=0)
        centroid_np = centroid

        # Coherence: mean cosine similarity of each vector to the centroid
        dots  = vecs @ centroid_np
        norms = np.linalg.norm(vecs, axis=1) * np.linalg.norm(centroid_np)
        norms = np.where(norms == 0, 1e-9, norms)
        coherence = float(np.mean(dots / norms))

        # Density: compactness of the cluster in the 2D embedding
        emb_cluster = embedding[mask]
        spread      = np.sqrt(((emb_cluster - emb_cluster.mean(axis=0)) ** 2).sum(axis=1)).mean()
        density     = float(1.0 / (1.0 + spread))

        vector_ids = [int(metadata[int(i)]["id"]) for i in indices]

        sources    = [metadata[int(i)].get("source", "") for i in indices]
        layers     = [metadata[int(i)].get("layer", "")  for i in indices]
        top_source = max(set(sources), key=sources.count) if sources else "unknown"
        top_layer  = max(set(layers),  key=layers.count)  if layers  else "surface"
        cluster_label = f"{top_layer} / {top_source}"

        clusters.append({
            "id":         int(label) + 1,
            "label":      cluster_label,
            "vector_ids": vector_ids,
            "centroid":   centroid.tolist(),
            "density":    density,
            "size":       int(len(indices)),
            "coherence":  coherence,
        })

    print(json.dumps({
        "clusters":      clusters,
        "noise_count":   int(np.sum(labels == -1)),
        "total_vectors": len(vectors),
    }))


if __name__ == "__main__":
    main()
