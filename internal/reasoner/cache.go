package reasoner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

type cachedReasoning struct {
	Fingerprint string           `json:"fingerprint"`
	CreatedAt   string           `json:"created_at"`
	Findings    []models.Finding `json:"findings"`
}

func TopologyFingerprint(
	collection string,
	clusterSeed int64,
	model string,
	budget ReasoningBudget,
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
) string {
	type clusterSummary struct {
		ID           int     `json:"id"`
		Size         int     `json:"size"`
		Density      float64 `json:"density"`
		Coherence    float64 `json:"coherence"`
		CentroidHash uint64  `json:"centroid_hash"`
	}
	type bridgeSummary struct {
		ClusterA int     `json:"cluster_a"`
		ClusterB int     `json:"cluster_b"`
		Strength float64 `json:"strength"`
	}
	type moatSummary struct {
		ClusterA int     `json:"cluster_a"`
		ClusterB int     `json:"cluster_b"`
		Distance float64 `json:"distance"`
	}

	clusterRows := make([]clusterSummary, 0, len(clusters))
	for _, cluster := range clusters {
		clusterRows = append(clusterRows, clusterSummary{
			ID:           cluster.ID,
			Size:         cluster.Size,
			Density:      roundFloat(cluster.Density, 4),
			Coherence:    roundFloat(cluster.Coherence, 4),
			CentroidHash: hashCentroid(cluster.Centroid),
		})
	}
	sort.Slice(clusterRows, func(i, j int) bool { return clusterRows[i].ID < clusterRows[j].ID })

	bridgeRows := make([]bridgeSummary, 0, len(bridges))
	for _, bridge := range bridges {
		a, b := bridge.ClusterA, bridge.ClusterB
		if a > b {
			a, b = b, a
		}
		bridgeRows = append(bridgeRows, bridgeSummary{
			ClusterA: a,
			ClusterB: b,
			Strength: roundFloat(bridge.Strength, 4),
		})
	}
	sort.Slice(bridgeRows, func(i, j int) bool {
		if bridgeRows[i].ClusterA != bridgeRows[j].ClusterA {
			return bridgeRows[i].ClusterA < bridgeRows[j].ClusterA
		}
		if bridgeRows[i].ClusterB != bridgeRows[j].ClusterB {
			return bridgeRows[i].ClusterB < bridgeRows[j].ClusterB
		}
		return bridgeRows[i].Strength > bridgeRows[j].Strength
	})

	moatRows := make([]moatSummary, 0, len(moats))
	for _, moat := range moats {
		a, b := moat.ClusterA, moat.ClusterB
		if a > b {
			a, b = b, a
		}
		moatRows = append(moatRows, moatSummary{
			ClusterA: a,
			ClusterB: b,
			Distance: roundFloat(moat.Distance, 4),
		})
	}
	sort.Slice(moatRows, func(i, j int) bool {
		if moatRows[i].ClusterA != moatRows[j].ClusterA {
			return moatRows[i].ClusterA < moatRows[j].ClusterA
		}
		if moatRows[i].ClusterB != moatRows[j].ClusterB {
			return moatRows[i].ClusterB < moatRows[j].ClusterB
		}
		return moatRows[i].Distance > moatRows[j].Distance
	})

	payload := struct {
		Collection  string           `json:"collection"`
		ClusterSeed int64            `json:"cluster_seed"`
		Model       string           `json:"model"`
		Budget      ReasoningBudget  `json:"budget"`
		Clusters    []clusterSummary `json:"clusters"`
		Bridges     []bridgeSummary  `json:"bridges"`
		Moats       []moatSummary    `json:"moats"`
	}{
		Collection:  collection,
		ClusterSeed: clusterSeed,
		Model:       model,
		Budget:      budget,
		Clusters:    clusterRows,
		Bridges:     bridgeRows,
		Moats:       moatRows,
	}

	serialized, _ := json.Marshal(payload)
	sum := sha256.Sum256(serialized)
	return hex.EncodeToString(sum[:])
}

func LoadCachedFindings(cacheDir, fingerprint string) ([]models.Finding, bool, error) {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s.json", fingerprint))
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var cached cachedReasoning
	if err := json.Unmarshal(payload, &cached); err != nil {
		return nil, false, err
	}
	if cached.Fingerprint != fingerprint {
		return nil, false, nil
	}
	return cached.Findings, true, nil
}

func SaveCachedFindings(cacheDir, fingerprint string, findings []models.Finding) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, fmt.Sprintf("%s.json", fingerprint))
	tmpPath := path + ".tmp"
	payload, err := json.MarshalIndent(cachedReasoning{
		Fingerprint: fingerprint,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Findings:    findings,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func roundFloat(value float64, places int) float64 {
	if places <= 0 {
		return math.Round(value)
	}
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func hashCentroid(centroid []float32) uint64 {
	hash := fnv.New64a()
	for _, value := range centroid {
		quantized := int32(math.Round(float64(value) * 10000))
		_, _ = hash.Write([]byte{byte(quantized), byte(quantized >> 8), byte(quantized >> 16), byte(quantized >> 24)})
	}
	return hash.Sum64()
}
