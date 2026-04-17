package reasoner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

const (
	defaultModel = "deepseek-reasoner" // full R1 chain-of-thought
	callTimeout  = 5 * time.Minute     // R1 can be slow; give it room
)

type Reasoner struct {
	apiURL string
	apiKey string
	model  string
	client *http.Client
}

func New(apiURL, apiKey string) *Reasoner {
	return New2(apiURL, apiKey, defaultModel)
}

func New2(apiURL, apiKey, model string) *Reasoner {
	return &Reasoner{
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: callTimeout},
	}
}

// ReasonAboutTopology uses DeepSeek to analyze vector topology.
// It reasons about every cluster, the top 10 bridges by strength,
// and the top 5 moats by distance.
// metadata is used to pull text snippets into cluster prompts; pass nil to omit.
func (r *Reasoner) ReasonAboutTopology(
	clusters []models.Cluster,
	bridges []models.Bridge,
	moats []models.Moat,
	metadata []models.VectorMetadata,
) []models.Finding {
	findings := []models.Finding{}
	total := len(clusters)

	// Build a fast lookup from vector ID → text fragment.
	byID := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		if m.Fragment != "" && m.Fragment != "N/A" {
			byID[m.ID] = m.Fragment
		}
	}

	sort.Slice(bridges, func(i, j int) bool { return bridges[i].Strength > bridges[j].Strength })
	if len(bridges) > 10 {
		bridges = bridges[:10]
	}
	sort.Slice(moats, func(i, j int) bool { return moats[i].Distance > moats[j].Distance })
	if len(moats) > 5 {
		moats = moats[:5]
	}
	total += len(bridges) + len(moats)

	done := 0

	logThinking := func(subject, thinking string) {
		if thinking == "" {
			return
		}
		fmt.Printf("\n\n   --- thinking: %s ---\n%s\n   ---\n", subject, thinking)
	}

	// Clusters
	firstPromptPrinted := false
	for _, cluster := range clusters {
		done++
		subject := fmt.Sprintf("Cluster %d: %s", cluster.ID, cluster.Label)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		snippets := clusterSnippets(cluster, byID, 5)
		prompt := buildClusterPrompt(cluster, snippets)
		if !firstPromptPrinted {
			fmt.Printf("\n\n--- R1 prompt (cluster %d) ---\n%s\n--- end prompt ---\n\n", cluster.ID, prompt)
			firstPromptPrinted = true
		}
		resp, err := r.callDeepSeek(prompt)
		if err != nil {
			fmt.Printf("\n   Warning: cluster %d: %v\n", cluster.ID, err)
			continue
		}
		logThinking(subject, resp.thinking)
		findings = append(findings, models.Finding{
			Type:           "cluster_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Confidence:     0.75,
			IsAnomaly:      cluster.Coherence < 0.5,
		})
	}

	// Top bridges
	for _, bridge := range bridges {
		done++
		subject := fmt.Sprintf("Bridge: %d ↔ %d", bridge.ClusterA, bridge.ClusterB)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		resp, err := r.callDeepSeek(buildBridgePrompt(bridge))
		if err != nil {
			continue
		}
		logThinking(subject, resp.thinking)
		findings = append(findings, models.Finding{
			Type:           "bridge_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Confidence:     0.75,
		})
	}

	// Top moats
	for _, moat := range moats {
		done++
		subject := fmt.Sprintf("Moat: %d ⊥ %d", moat.ClusterA, moat.ClusterB)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		resp, err := r.callDeepSeek(buildMoatPrompt(moat))
		if err != nil {
			continue
		}
		logThinking(subject, resp.thinking)
		findings = append(findings, models.Finding{
			Type:           "moat_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Confidence:     0.75,
			IsAnomaly:      true,
		})
	}

	fmt.Printf("\r   ✓ reasoning complete (%d/%d)                              \n", done, total)
	return findings
}

type deepSeekResponse struct {
	thinking   string
	conclusion string
}

func (r *Reasoner) callDeepSeek(prompt string) (*deepSeekResponse, error) {
	reqBody := map[string]interface{}{
		"model": r.model,
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
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bad JSON response: %w", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response: %s", string(body))
	}
	msg, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected message format")
	}

	conclusion, _ := msg["content"].(string)
	thinking, _   := msg["reasoning_content"].(string)

	return &deepSeekResponse{thinking: thinking, conclusion: conclusion}, nil
}

// formatForReport combines the visible thinking chain and conclusion into the
// markdown stored in the report.
func formatForReport(r *deepSeekResponse) string {
	if r.thinking != "" {
		return fmt.Sprintf("**Thinking:**\n%s\n\n**Conclusion:**\n%s", r.thinking, r.conclusion)
	}
	return r.conclusion
}

func buildClusterPrompt(cluster models.Cluster, snippets []string) string {
	base := fmt.Sprintf(`Analyze this vector embedding cluster from a knowledge archaeology system.

Cluster Details:
- ID: %d
- Label: %s
- Size: %d vectors
- Density: %.2f
- Coherence: %.2f
`, cluster.ID, cluster.Label, cluster.Size, cluster.Density, cluster.Coherence)

	if len(snippets) > 0 {
		base += "\nSample content from member vectors:\n"
		for _, s := range snippets {
			base += fmt.Sprintf("• %s\n", s)
		}
	}

	base += "\nIn 2-3 sentences: what semantic concept does this cluster represent? End your response with a **Conclusion:** paragraph naming the concept."
	return base
}

// clusterSnippets returns up to n non-empty text fragments from cluster members.
func clusterSnippets(cluster models.Cluster, byID map[uint64]string, n int) []string {
	var out []string
	for _, id := range cluster.VectorIDs {
		if frag, ok := byID[id]; ok {
			out = append(out, frag)
			if len(out) >= n {
				break
			}
		}
	}
	return out
}

func buildBridgePrompt(bridge models.Bridge) string {
	return fmt.Sprintf(`Analyze this semantic bridge between vector clusters:

Strength: %.2f (%s)
Connecting: Cluster %d ↔ Cluster %d

In 1-2 sentences: why does this connection exist?`,
		bridge.Strength, bridge.LinkType, bridge.ClusterA, bridge.ClusterB)
}

func buildMoatPrompt(moat models.Moat) string {
	return fmt.Sprintf(`Analyze this knowledge moat (isolation) between vector clusters:

Distance: %.2f
Isolated: Cluster %d ⊥ Cluster %d

In 1-2 sentences: why is there no semantic connection?`,
		moat.Distance, moat.ClusterA, moat.ClusterB)
}
