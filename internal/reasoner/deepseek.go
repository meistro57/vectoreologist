package reasoner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/meistro57/vectoreologist/internal/models"
)

type Reasoner struct {
	apiURL string
	apiKey string
	client *http.Client
}

func New(apiURL, apiKey string) *Reasoner {
	return &Reasoner{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// ReasonAboutTopology uses DeepSeek R1 to analyze vector topology
func (r *Reasoner) ReasonAboutTopology(
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
) []models.Finding {
	findings := []models.Finding{}

	// Reason about each cluster
	for _, cluster := range clusters {
		prompt := buildClusterPrompt(cluster)
		reasoning, err := r.callDeepSeek(prompt)
		if err != nil {
			fmt.Printf("Warning: Failed to reason about cluster %d: %v\n", cluster.ID, err)
			continue
		}

		findings = append(findings, models.Finding{
			Type:           "cluster_analysis",
			Subject:        fmt.Sprintf("Cluster %d", cluster.ID),
			ReasoningChain: reasoning,
			Confidence:     0.75,
			IsAnomaly:      cluster.Coherence < 0.5,
		})
	}

	// Reason about bridges
	for _, bridge := range bridges {
		prompt := buildBridgePrompt(bridge)
		reasoning, err := r.callDeepSeek(prompt)
		if err != nil {
			continue
		}

		findings = append(findings, models.Finding{
			Type:           "bridge_analysis",
			Subject:        fmt.Sprintf("Bridge: %d ↔ %d", bridge.ClusterA, bridge.ClusterB),
			ReasoningChain: reasoning,
			Confidence:     0.75,
		})
	}

	// Reason about moats
	for _, moat := range moats {
		prompt := buildMoatPrompt(moat)
		reasoning, err := r.callDeepSeek(prompt)
		if err != nil {
			continue
		}

		findings = append(findings, models.Finding{
			Type:           "moat_analysis",
			Subject:        fmt.Sprintf("Moat: %d ⊥ %d", moat.ClusterA, moat.ClusterB),
			ReasoningChain: reasoning,
			Confidence:     0.75,
			IsAnomaly:      true,
		})
	}

	return findings
}

func (r *Reasoner) callDeepSeek(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": "deepseek-reasoner",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", r.apiURL+"/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	choices := result["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	return message["content"].(string), nil
}

func buildClusterPrompt(cluster models.Cluster) string {
	return fmt.Sprintf(`Analyze this vector embedding cluster from a knowledge archaeology system.

Cluster Details:
- ID: %d
- Label: %s
- Size: %d vectors
- Density: %.2f
- Coherence: %.2f

What semantic concept does this cluster represent? Provide visible reasoning.`, 
		cluster.ID, cluster.Label, cluster.Size, cluster.Density, cluster.Coherence)
}

func buildBridgePrompt(bridge models.Bridge) string {
	return fmt.Sprintf(`Analyze this semantic bridge between vector clusters:

Strength: %.2f (%s)
Connecting: Cluster %d ↔ Cluster %d

Why does this connection exist?`, bridge.Strength, bridge.LinkType, bridge.ClusterA, bridge.ClusterB)
}

func buildMoatPrompt(moat models.Moat) string {
	return fmt.Sprintf(`Analyze this knowledge moat (isolation):

Distance: %.2f
Isolated: Cluster %d ⊥ Cluster %d

Why is there no semantic connection?`, moat.Distance, moat.ClusterA, moat.ClusterB)
}
