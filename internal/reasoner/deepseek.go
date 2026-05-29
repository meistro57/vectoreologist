package reasoner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/meistro57/vectoreologist/internal/models"
)

const (
	defaultModel = "deepseek-reasoner" // full R1 chain-of-thought
	callTimeout  = 5 * time.Minute     // R1 can be slow; give it room
)

type Reasoner struct {
	apiURL         string
	apiKey         string
	model          string
	client         *http.Client
	budget         ReasoningBudget
	findingHandler func(models.Finding, int, int)
}

func New(apiURL, apiKey string) *Reasoner {
	return New2(apiURL, apiKey, defaultModel)
}

func New2(apiURL, apiKey, model string) *Reasoner {
	return &Reasoner{
		apiURL: strings.TrimRight(apiURL, "/"),
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: callTimeout},
		budget: BudgetForProfile("balanced"),
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
	selectedClusters := applyClusterBudget(clusters, r.budget.MaxClusters)
	selectedBridges := applyBridgeBudget(bridges, r.budget.MaxBridges)
	selectedMoats := applyMoatBudget(moats, r.budget.MaxMoats)
	total := len(selectedClusters) + len(selectedBridges) + len(selectedMoats)

	byID := make(map[uint64]string, len(metadata))
	for _, m := range metadata {
		if m.Fragment != "" && m.Fragment != "N/A" {
			byID[m.ID] = m.Fragment
		}
	}

	done := 0

	logThinking := func(subject, thinking string) {
		if thinking == "" {
			return
		}
		fmt.Printf("\n\n   --- thinking: %s ---\n%s\n   ---\n", subject, thinking)
	}

	// Clusters
	firstPromptPrinted := false
	for _, cluster := range selectedClusters {
		done++
		subject := fmt.Sprintf("Cluster %d: %s", cluster.ID, cluster.Label)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		snippets := clusterSnippets(cluster, byID, 8)
		prompt := buildClusterPrompt(cluster, snippets)
		if !firstPromptPrinted {
			fmt.Printf("\n\n--- R1 prompt (cluster %d) ---\n%s\n--- end prompt ---\n\n", cluster.ID, prompt)
			firstPromptPrinted = true
		}
		resp, err := r.callDeepSeekWithSubject(subject, prompt)
		if err != nil {
			fmt.Printf("\n   Warning: cluster %d: %v\n", cluster.ID, err)
			continue
		}
		if !resp.streamed {
			logThinking(subject, resp.thinking)
		}
		finding := models.Finding{
			Type:           "cluster_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Evidence:       strings.Join(snippets, "\n"),
			Confidence:     0.75,
			IsAnomaly:      cluster.Coherence < 0.5,
			Clusters:       []int{cluster.ID},
		}
		findings = append(findings, finding)
		if r.findingHandler != nil {
			r.findingHandler(finding, done, total)
		}
	}

	// Top bridges
	for _, bridge := range selectedBridges {
		done++
		subject := fmt.Sprintf("Bridge: %d ↔ %d", bridge.ClusterA, bridge.ClusterB)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		aSnips, bSnips := bridgeSnippets(bridge, byID, 4)
		resp, err := r.callDeepSeekWithSubject(subject, buildBridgePrompt(bridge, aSnips, bSnips))
		if err != nil {
			continue
		}
		if !resp.streamed {
			logThinking(subject, resp.thinking)
		}
		finding := models.Finding{
			Type:           "bridge_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Evidence:       encodeBridgeEvidence(aSnips, bSnips),
			Confidence:     0.75,
			Clusters:       []int{bridge.ClusterA, bridge.ClusterB},
		}
		findings = append(findings, finding)
		if r.findingHandler != nil {
			r.findingHandler(finding, done, total)
		}
	}

	// Top moats
	for _, moat := range selectedMoats {
		done++
		subject := fmt.Sprintf("Moat: %d ⊥ %d", moat.ClusterA, moat.ClusterB)
		fmt.Printf("\r   reasoning %d/%d: %s ...", done, total, subject)
		resp, err := r.callDeepSeekWithSubject(subject, buildMoatPrompt(moat))
		if err != nil {
			continue
		}
		if !resp.streamed {
			logThinking(subject, resp.thinking)
		}
		finding := models.Finding{
			Type:           "moat_analysis",
			Subject:        subject,
			ReasoningChain: formatForReport(resp),
			Confidence:     0.75,
			IsAnomaly:      true,
		}
		findings = append(findings, finding)
		if r.findingHandler != nil {
			r.findingHandler(finding, done, total)
		}
	}

	fmt.Printf("\r   ✓ reasoning complete (%d/%d)                              \n", done, total)
	return findings
}

type deepSeekResponse struct {
	thinking   string
	conclusion string
	streamed   bool
}

func (r *Reasoner) callDeepSeek(prompt string) (*deepSeekResponse, error) {
	return r.callDeepSeekWithSubject("", prompt)
}

func (r *Reasoner) callDeepSeekWithSubject(subject, prompt string) (*deepSeekResponse, error) {
	reqBody := map[string]interface{}{
		"model": r.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0,
		"stream":      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequest("POST", r.apiURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("deepseek API error %s", resp.Status)
		}
		return nil, fmt.Errorf("deepseek API error %s: %s", resp.Status, string(body))
	}

	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		streamed, err := r.readStreamedResponse(resp.Body, subject)
		if err != nil {
			return nil, err
		}
		return streamed, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return parseJSONResponse(body)
}

func parseJSONResponse(body []byte) (*deepSeekResponse, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("bad JSON response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response: %s", string(body))
	}
	return &deepSeekResponse{
		thinking:   result.Choices[0].Message.ReasoningContent,
		conclusion: result.Choices[0].Message.Content,
		streamed:   false,
	}, nil
}

func (r *Reasoner) readStreamedResponse(body io.Reader, subject string) (*deepSeekResponse, error) {
	type streamChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var thinking strings.Builder
	var conclusion strings.Builder
	streamPrinted := false
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read stream: %w", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				break
			}
			if payload != "" {
				var chunk streamChunk
				if unmarshalErr := json.Unmarshal([]byte(payload), &chunk); unmarshalErr == nil && len(chunk.Choices) > 0 {
					thinkingDelta := chunk.Choices[0].Delta.ReasoningContent
					conclusionDelta := chunk.Choices[0].Delta.Content
					if thinkingDelta == "" && conclusionDelta == "" {
						thinkingDelta = chunk.Choices[0].Message.ReasoningContent
						conclusionDelta = chunk.Choices[0].Message.Content
					}
					if thinkingDelta != "" {
						thinking.WriteString(thinkingDelta)
						if subject != "" {
							if !streamPrinted {
								fmt.Printf("\n\n   --- stream: %s ---\n", subject)
								streamPrinted = true
							}
							fmt.Print(thinkingDelta)
						}
					}
					if conclusionDelta != "" {
						conclusion.WriteString(conclusionDelta)
						if subject != "" {
							if !streamPrinted {
								fmt.Printf("\n\n   --- stream: %s ---\n", subject)
								streamPrinted = true
							}
							fmt.Print(conclusionDelta)
						}
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	if streamPrinted {
		fmt.Printf("\n   --- end stream ---\n")
	}
	if strings.TrimSpace(thinking.String()) == "" && strings.TrimSpace(conclusion.String()) == "" {
		return nil, fmt.Errorf("empty streamed response")
	}
	return &deepSeekResponse{
		thinking:   thinking.String(),
		conclusion: conclusion.String(),
		streamed:   true,
	}, nil
}

// formatForReport keeps only final-facing model output.
func formatForReport(r *deepSeekResponse) string {
	return strings.TrimSpace(r.conclusion)
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

// clusterSnippets returns up to n unique, non-empty text fragments from cluster
// members. It samples evenly across the cluster rather than taking the first N,
// and deduplicates to avoid feeding identical lines to the reasoner.
func clusterSnippets(cluster models.Cluster, byID map[uint64]string, n int) []string {
	ids := cluster.VectorIDs
	if len(ids) == 0 {
		return nil
	}

	// Step through the cluster at even intervals to get diversity.
	step := len(ids) / (n * 3) // oversample 3x to account for dupes/missing
	if step < 1 {
		step = 1
	}

	seen := make(map[string]bool)
	var out []string
	for i := 0; i < len(ids) && len(out) < n; i += step {
		if frag, ok := byID[ids[i]]; ok {
			// Truncate long fragments for the prompt.
			if len(frag) > 200 {
				frag = frag[:200] + "..."
			}
			if !seen[frag] {
				seen[frag] = true
				out = append(out, frag)
			}
		}
	}

	// If striding missed enough unique snippets, do a second pass.
	if len(out) < n {
		for _, id := range ids {
			if len(out) >= n {
				break
			}
			if frag, ok := byID[id]; ok {
				if len(frag) > 200 {
					frag = frag[:200] + "..."
				}
				if !seen[frag] {
					seen[frag] = true
					out = append(out, frag)
				}
			}
		}
	}

	return out
}

func bridgeSnippets(bridge models.Bridge, byID map[uint64]string, maxPerSide int) ([]string, []string) {
	var aSnips, bSnips []string
	seenA := make(map[string]bool)
	seenB := make(map[string]bool)
	for _, sl := range bridge.SampleLinks {
		if t, ok := byID[sl.ChunkAID]; ok && !seenA[t] {
			aSnips = append(aSnips, truncateSnippet(t, 200))
			seenA[t] = true
		}
		if t, ok := byID[sl.ChunkBID]; ok && !seenB[t] {
			bSnips = append(bSnips, truncateSnippet(t, 200))
			seenB[t] = true
		}
		if len(aSnips) >= maxPerSide && len(bSnips) >= maxPerSide {
			break
		}
	}
	return aSnips, bSnips
}

func truncateSnippet(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func encodeBridgeEvidence(aSnips, bSnips []string) string {
	var sb strings.Builder
	sb.WriteString("A:\n")
	for _, s := range aSnips {
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	sb.WriteString("B:\n")
	for _, s := range bSnips {
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func buildBridgePrompt(bridge models.Bridge, aSnips, bSnips []string) string {
	prompt := fmt.Sprintf("Analyze this semantic bridge between vector clusters:\n\nStrength: %.2f (%s)\nCluster %d ↔ Cluster %d\n",
		bridge.Strength, bridge.LinkType, bridge.ClusterA, bridge.ClusterB)

	if len(aSnips) > 0 {
		prompt += fmt.Sprintf("\nCluster %d samples:\n• %s\n", bridge.ClusterA, strings.Join(aSnips, "\n• "))
	}
	if len(bSnips) > 0 {
		prompt += fmt.Sprintf("\nCluster %d samples:\n• %s\n", bridge.ClusterB, strings.Join(bSnips, "\n• "))
	}

	prompt += "\nIn 2-3 sentences: what shared concept bridges these two clusters? End with a **Conclusion:** naming the bridge concept."
	return prompt
}

func buildMoatPrompt(moat models.Moat) string {
	return fmt.Sprintf(`Analyze this knowledge moat (isolation) between vector clusters:

Distance: %.2f
Isolated: Cluster %d ⊥ Cluster %d

In 1-2 sentences: why is there no semantic connection?`,
		moat.Distance, moat.ClusterA, moat.ClusterB)
}
