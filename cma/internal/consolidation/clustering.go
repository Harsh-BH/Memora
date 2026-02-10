package consolidation

import (
	"math"

	"github.com/memora/cma/internal/models"
)

// DBSCAN implements Density-Based Spatial Clustering of Applications with Noise
// over episode embeddings for the consolidation engine.
//
// During the Sleep cycle, episodic fragments are clustered by vector similarity
// to identify groups of semantically related memories that should be merged
// into semantic propositions (knowledge graph facts).
type DBSCAN struct {
	epsilon   float64 // distance threshold for neighborhood
	minPoints int     // minimum points to form a cluster
}

// NewDBSCAN creates a new DBSCAN clusterer.
func NewDBSCAN(epsilon float64, minPoints int) *DBSCAN {
	if epsilon <= 0 {
		epsilon = 0.3
	}
	if minPoints <= 0 {
		minPoints = 3
	}
	return &DBSCAN{
		epsilon:   epsilon,
		minPoints: minPoints,
	}
}

// Cluster groups episodes by embedding similarity using DBSCAN.
// Returns a slice of Cluster structs. Noise points (not assigned to any cluster)
// are returned as singleton clusters.
func (d *DBSCAN) Cluster(episodes []models.Episode) []models.Cluster {
	n := len(episodes)
	if n == 0 {
		return nil
	}

	labels := make([]int, n)  // -1 = unvisited, 0 = noise, >0 = cluster ID
	for i := range labels {
		labels[i] = -1
	}

	clusterID := 0

	for i := 0; i < n; i++ {
		if labels[i] != -1 {
			continue // already processed
		}

		neighbors := d.regionQuery(episodes, i)
		if len(neighbors) < d.minPoints {
			labels[i] = 0 // noise
			continue
		}

		clusterID++
		labels[i] = clusterID

		// Expand cluster.
		seedSet := make(map[int]bool)
		for _, idx := range neighbors {
			seedSet[idx] = true
		}
		delete(seedSet, i)

		queue := make([]int, 0, len(neighbors))
		for _, idx := range neighbors {
			if idx != i {
				queue = append(queue, idx)
			}
		}

		for len(queue) > 0 {
			q := queue[0]
			queue = queue[1:]

			if labels[q] == 0 {
				labels[q] = clusterID // change noise to border point
			}
			if labels[q] != -1 {
				continue // already processed
			}

			labels[q] = clusterID

			qNeighbors := d.regionQuery(episodes, q)
			if len(qNeighbors) >= d.minPoints {
				for _, idx := range qNeighbors {
					if !seedSet[idx] {
						seedSet[idx] = true
						queue = append(queue, idx)
					}
				}
			}
		}
	}

	// Build cluster objects.
	clusterMap := make(map[int][]models.Episode)
	for i, label := range labels {
		if label <= 0 {
			// Noise points become singleton clusters.
			clusterMap[-(i + 1)] = []models.Episode{episodes[i]}
		} else {
			clusterMap[label] = append(clusterMap[label], episodes[i])
		}
	}

	var clusters []models.Cluster
	for id, eps := range clusterMap {
		centroid := computeCentroid(eps)
		clusters = append(clusters, models.Cluster{
			ID:       id,
			Episodes: eps,
			Centroid: centroid,
		})
	}

	return clusters
}

// regionQuery finds all episodes within epsilon distance of the i-th episode.
func (d *DBSCAN) regionQuery(episodes []models.Episode, i int) []int {
	var neighbors []int
	for j := 0; j < len(episodes); j++ {
		dist := cosineDistance(episodes[i].Embedding, episodes[j].Embedding)
		if dist <= d.epsilon {
			neighbors = append(neighbors, j)
		}
	}
	return neighbors
}

// cosineDistance computes 1 - cosine_similarity between two vectors.
func cosineDistance(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 1.0
	}

	similarity := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	return 1.0 - similarity
}

// computeCentroid calculates the mean embedding vector of a cluster.
func computeCentroid(episodes []models.Episode) []float32 {
	if len(episodes) == 0 {
		return nil
	}

	dim := len(episodes[0].Embedding)
	if dim == 0 {
		return nil
	}

	centroid := make([]float32, dim)
	for _, ep := range episodes {
		for i, v := range ep.Embedding {
			centroid[i] += v
		}
	}

	n := float32(len(episodes))
	for i := range centroid {
		centroid[i] /= n
	}

	return centroid
}
