package synthesis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

type reportData struct {
	findings   []models.Finding
	clusters   []models.Cluster
	bridges    []models.Bridge
	moats      []models.Moat
	metadata   []models.VectorMetadata
	collection string
}

type clusterSection struct {
	cluster         models.Cluster
	analysis        string
	suggestedLabel  string
	sourceBalance   map[string]float64
	snippets        []string
	flags           []string
	confidence      float64
	topic           string
	mode            string
	posture         string
	finalConcept    string
	duplicateHeavy  bool
	overSampled      bool
}

type bridgeSection struct {
	bridge       models.Bridge
	labelA       string
	labelB       string
	evidenceA    []string
	evidenceB    []string
	sharedConcept string
	analysis     string
	conclusion   string
	flags        []string
	confidence   float64
	skipped      bool
}

type semanticAttractor struct {
	name       string
	clusters   []int
	bridges    []string
	confidence float64
	explanation string
}

type reportDiagnostics struct {
	duplicateHeavyClusters int
	overSampledClusters    int
	skippedBridges         int
}

type topologyDiagnostics struct {
	subject string
	reason  string
}

type reportArtifact struct {
	data              reportData
	clusterSections   []clusterSection
	bridgeSections    []bridgeSection
	attractors        []semanticAttractor
	reviewItems       []string
	recommendations   []string
	diagnostics       reportDiagnostics
	topologyDiagnostics topologyDiagnostics
}

func buildReportArtifact(data reportData) reportArtifact {
	clusterFindings := clusterFindingsByID(data.findings)
	bridgeFindings := bridgeFindingsByPair(data.findings)
	anomalyFlags := anomalyFlagsByCluster(data.findings)
	metadataByID := metadataByID(data.metadata)
	clusterByID := clusterByID(data.clusters)

	clusterSections := make([]clusterSection, 0, len(data.clusters))
	for _, cluster := range data.clusters {
		clusterSections = append(clusterSections, renderCluster(cluster, metadataByID, clusterFindings[cluster.ID], anomalyFlags[cluster.ID]))
	}

	bridgeSections := make([]bridgeSection, 0, len(data.bridges))
	for _, bridge := range data.bridges {
		bridgeSections = append(bridgeSections, renderBridge(bridge, clusterByID, metadataByID, bridgeFindings[bridgeKey(bridge.ClusterA, bridge.ClusterB)]))
	}

	return reportArtifact{
		data:                data,
		clusterSections:     clusterSections,
		bridgeSections:      bridgeSections,
		attractors:          renderSemanticAttractors(clusterSections, bridgeSections),
		reviewItems:         recommendedReviewItems(clusterSections, bridgeSections),
		recommendations:     buildRecommendations(clusterSections, bridgeSections),
		diagnostics:         summarizeDiagnostics(clusterSections, bridgeSections),
		topologyDiagnostics: extractTopologyDiagnostics(data.findings),
	}
}

func renderReport(data reportData) string {
	return renderMarkdown(buildReportArtifact(data))
}

func renderMarkdown(artifact reportArtifact) string {
	data := artifact.data
	var sb strings.Builder
	sb.WriteString("# Vectoreology Report\n\n")
	sb.WriteString(renderExecutiveSummary(data, artifact.diagnostics, artifact.attractors, artifact.reviewItems, artifact.topologyDiagnostics))
	sb.WriteString("## Cluster Analysis\n\n")
	for _, section := range artifact.clusterSections {
		sb.WriteString(renderClusterMarkdown(section))
	}

	sb.WriteString("## Semantic Bridges\n\n")
	for _, section := range artifact.bridgeSections {
		sb.WriteString(renderBridgeMarkdown(section))
	}

	sb.WriteString("## Knowledge Moats\n\n")
	if len(data.moats) == 0 {
		sb.WriteString("No high-distance moat pairs were detected.\n\n")
	} else {
		for _, moat := range data.moats {
			sb.WriteString(fmt.Sprintf("### Moat: %d ⊥ %d\n\n", moat.ClusterA, moat.ClusterB))
			sb.WriteString(fmt.Sprintf("Distance: %.2f\n\n", moat.Distance))
			sb.WriteString(fmt.Sprintf("Explanation: %s\n\n", cleanLLMOutput(moat.Explanation)))
		}
	}

	sb.WriteString("## Semantic Attractors\n\n")
	if len(artifact.attractors) == 0 {
		sb.WriteString("No recurring attractors reached minimum support.\n\n")
	} else {
		for i, attr := range artifact.attractors {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, attr.name))
			sb.WriteString(fmt.Sprintf("   - Supporting clusters: %s\n", joinInts(attr.clusters)))
			sb.WriteString(fmt.Sprintf("   - Supporting bridges: %s\n", joinStrings(attr.bridges)))
			sb.WriteString(fmt.Sprintf("   - Confidence score: %.2f\n", attr.confidence))
			sb.WriteString(fmt.Sprintf("   - %s\n\n", attr.explanation))
		}
	}

	sb.WriteString("## Recommendations\n\n")
	for _, rec := range artifact.recommendations {
		sb.WriteString(fmt.Sprintf("- %s\n", rec))
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderExecutiveSummary(data reportData, diagnostics reportDiagnostics, attractors []semanticAttractor, reviewItems []string, topo topologyDiagnostics) string {
	var sb strings.Builder
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("- Total clusters: %d\n", len(data.clusters)))
	sb.WriteString(fmt.Sprintf("- Total bridges: %d\n", len(data.bridges)))
	sb.WriteString(fmt.Sprintf("- Total moats: %d\n", len(data.moats)))
	sb.WriteString(fmt.Sprintf("- Duplicate-heavy clusters: %d\n", diagnostics.duplicateHeavyClusters))
	sb.WriteString(fmt.Sprintf("- Source-oversampled clusters: %d\n", diagnostics.overSampledClusters))
	sb.WriteString(fmt.Sprintf("- Bridge interpretations skipped (insufficient evidence): %d\n", diagnostics.skippedBridges))
	if strings.TrimSpace(topo.subject) != "" {
		sb.WriteString(fmt.Sprintf("- DBSCAN diagnostics: %s\n", topo.subject))
		if strings.TrimSpace(topo.reason) != "" {
			sb.WriteString(fmt.Sprintf("- DBSCAN rationale: %s\n", topo.reason))
		}
	}
	sb.WriteString("\n")
	sb.WriteString("Top 3 strongest semantic attractors across the corpus:\n")
	for i := 0; i < min(3, len(attractors)); i++ {
		sb.WriteString(fmt.Sprintf("%d. %s (%.2f)\n", i+1, attractors[i].name, attractors[i].confidence))
	}
	if len(attractors) == 0 {
		sb.WriteString("1. No attractor reached minimum support\n")
	}
	sb.WriteString("\nTop 3 recommended human review items:\n")
	if len(reviewItems) == 0 {
		sb.WriteString("1. No high-priority review flags detected\n\n")
		return sb.String()
	}
	for i := 0; i < min(3, len(reviewItems)); i++ {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, reviewItems[i]))
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderCluster(cluster models.Cluster, metadataByID map[uint64]models.VectorMetadata, finding models.Finding, anomalyFlags []string) clusterSection {
	sourceBalance := calculateSourceBalance(cluster, metadataByID)
	snippets := representativeSnippets(cluster, metadataByID, 3)
	duplicateHeavy := detectDuplicateHeavyCluster(cluster)
	overSampled := detectSourceOversampling(sourceBalance)
	cleaned := cleanLLMOutput(finding.ReasoningChain)
	cleaned = collapseConclusionBlocks(cleaned)
	hasFinding := strings.TrimSpace(cleaned) != ""
	finalConcept := ""
	if hasFinding {
		finalConcept = extractFinalConcept(cleaned)
	}
	confidence := clusterConfidence(cluster, finding)
	topic, mode, posture := taxonomyFields(cluster)
	suggested := generateSuggestedLabel(cluster, snippets, finalConcept)

	flags := append([]string{}, anomalyFlags...)
	if duplicateHeavy {
		flags = append(flags, "Potential near-duplicate cluster. Human review recommended before interpretation.")
	}
	if overSampled {
		flags = append(flags, "Source oversampling detected. Interpretive confidence may be inflated.")
	}

	analysis := stripTrailingEcho(cleaned, finalConcept)
	switch {
	case !validateInterpretationInputs(snippets):
		analysis = "Insufficient evidence to interpret this cluster."
		finalConcept = "Insufficient evidence"
		flags = append(flags, "Insufficient representative snippets for interpretation")
	case !hasFinding:
		analysis = "No interpretive analysis available for this cluster."
		finalConcept = ""
		flags = append(flags, "Awaiting interpretive analysis")
	}
	if duplicateHeavy {
		flags = append(flags, "Low-trust interpretation due to near-duplicate structure")
	}
	if len(flags) == 0 {
		flags = append(flags, "None")
	}

	return clusterSection{
		cluster:         cluster,
		analysis:        analysis,
		suggestedLabel:  suggested,
		sourceBalance:   sourceBalance,
		snippets:        snippets,
		flags:           dedupeStrings(flags),
		confidence:      confidence,
		topic:           topic,
		mode:            mode,
		posture:         posture,
		finalConcept:    finalConcept,
		duplicateHeavy:  duplicateHeavy,
		overSampled:      overSampled,
	}
}

func renderClusterMarkdown(section clusterSection) string {
	machineLabel := section.cluster.Source
	if machineLabel == "" {
		machineLabel = section.cluster.Label
	}
	var sb strings.Builder

	// Duplicate-heavy (noise) clusters: emit a compact stub — no source list, no evidence wall.
	if section.duplicateHeavy {
		sb.WriteString(fmt.Sprintf("### Cluster %d ⚠ NOISE — %s\n\n", section.cluster.ID, machineLabel))
		sb.WriteString(fmt.Sprintf("Size: %d | Density: %.2f | Coherence: %.2f\n\n", section.cluster.Size, section.cluster.Density, section.cluster.Coherence))
		sb.WriteString("Review Flags:\n")
		for _, flag := range section.flags {
			sb.WriteString(fmt.Sprintf("- %s\n", flag))
		}
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("### Cluster %d: %s\n\n", section.cluster.ID, shortenForHeading(section.suggestedLabel, 100)))
	sb.WriteString(fmt.Sprintf("Machine Label: `%s`  \n", machineLabel))
	sb.WriteString(fmt.Sprintf("Size: %d  \n", section.cluster.Size))
	sb.WriteString(fmt.Sprintf("Density: %.2f  \n", section.cluster.Density))
	sb.WriteString(fmt.Sprintf("Coherence: %.2f  \n", section.cluster.Coherence))
	sb.WriteString(fmt.Sprintf("Confidence: %.2f\n\n", section.confidence))

	sb.WriteString("Taxonomy:\n")
	sb.WriteString(fmt.Sprintf("- Topic: %s\n", section.topic))
	sb.WriteString(fmt.Sprintf("- Mode: %s\n", section.mode))
	sb.WriteString(fmt.Sprintf("- Posture: %s\n\n", section.posture))

	sb.WriteString("Source Balance:\n")
	for _, line := range sourceBalanceLines(section.sourceBalance) {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\nRepresentative Evidence:\n")
	if len(section.snippets) == 0 {
		sb.WriteString("1. \"Insufficient evidence\"\n\n")
	} else {
		for i, snip := range section.snippets {
			sb.WriteString(fmt.Sprintf("%d. \"%s\"\n", i+1, snip))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Analysis:\n")
	sb.WriteString(section.analysis)
	sb.WriteString("\n\n")

	sb.WriteString("Conclusion:\n")
	if strings.TrimSpace(section.finalConcept) == "" {
		sb.WriteString("Pending interpretive analysis")
	} else {
		sb.WriteString(section.finalConcept)
	}
	sb.WriteString("\n\n")

	sb.WriteString("Review Flags:\n")
	for _, flag := range section.flags {
		sb.WriteString(fmt.Sprintf("- %s\n", flag))
	}
	sb.WriteString("\n")
	return sb.String()
}

// minBridgeSnippets is the minimum number of representative snippets required
// on EACH side of a bridge before it qualifies for LLM interpretation.
// A bridge with fewer than this on either side is skipped — dual-sided enforcement.
const minBridgeSnippets = 2

func renderBridge(bridge models.Bridge, clustersByID map[int]models.Cluster, metadataByID map[uint64]models.VectorMetadata, finding models.Finding) bridgeSection {
	clusterA := clustersByID[bridge.ClusterA]
	clusterB := clustersByID[bridge.ClusterB]
	labelA := generateSuggestedLabel(clusterA, representativeSnippets(clusterA, metadataByID, 3), "")
	labelB := generateSuggestedLabel(clusterB, representativeSnippets(clusterB, metadataByID, 3), "")
	evidenceA, evidenceB := bridgeEvidence(bridge, metadataByID)
	cleaned := cleanLLMOutput(finding.ReasoningChain)
	cleaned = collapseConclusionBlocks(cleaned)
	hasFinding := strings.TrimSpace(cleaned) != ""
	sharedConcept := bridgeSharedConcept(bridge, cleaned)
	confidence := bridgeConfidence(bridge, finding)
	flags := []string{}
	skipped := false
	analysis := stripTrailingEcho(cleaned, sharedConcept)

	// Dual-sided snippet enforcement: both sides must meet the minimum threshold.
	// A bridge where one side is empty OR under-evidenced is not ready for interpretation.
	aInsufficient := len(evidenceA) < minBridgeSnippets
	bInsufficient := len(evidenceB) < minBridgeSnippets

	switch {
	case aInsufficient || bInsufficient:
		skipped = true
		analysis = "Insufficient evidence to interpret this bridge."
		sharedConcept = "Insufficient evidence to interpret this bridge."
		if aInsufficient && bInsufficient {
			flags = append(flags, fmt.Sprintf("Insufficient representative snippets on both sides (A: %d, B: %d, need ≥%d each)", len(evidenceA), len(evidenceB), minBridgeSnippets))
		} else if aInsufficient {
			flags = append(flags, fmt.Sprintf("Insufficient representative snippets on cluster A side (%d, need ≥%d)", len(evidenceA), minBridgeSnippets))
		} else {
			flags = append(flags, fmt.Sprintf("Insufficient representative snippets on cluster B side (%d, need ≥%d)", len(evidenceB), minBridgeSnippets))
		}
	case !hasFinding:
		analysis = "No interpretive analysis available for this bridge."
		sharedConcept = "Pending interpretive analysis"
		flags = append(flags, "Awaiting interpretive analysis")
	}
	if len(flags) == 0 {
		flags = append(flags, "None")
	}

	return bridgeSection{
		bridge:       bridge,
		labelA:       labelA,
		labelB:       labelB,
		evidenceA:    evidenceA,
		evidenceB:    evidenceB,
		sharedConcept: sharedConcept,
		analysis:     limitSentences(analysis, 4),
		conclusion:   sharedConcept,
		flags:        flags,
		confidence:   confidence,
		skipped:      skipped,
	}
}

func renderBridgeMarkdown(section bridgeSection) string {
	var sb strings.Builder

	// Pending/skipped bridges: compact stub — no evidence walls, no triple-repeated placeholder.
	isPending := section.sharedConcept == "Pending interpretive analysis" ||
		section.sharedConcept == "Shared concept unavailable" ||
		strings.Contains(strings.ToLower(section.analysis), "no interpretive analysis available")
	if section.skipped || isPending {
		sb.WriteString(fmt.Sprintf("### Bridge: Cluster %d ↔ Cluster %d — strength %.2f — ⏳ pending\n\n",
			section.bridge.ClusterA, section.bridge.ClusterB, section.bridge.Strength))
		for _, flag := range section.flags {
			if flag != "None" {
				sb.WriteString(fmt.Sprintf("- %s\n", flag))
			}
		}
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("### Bridge: Cluster %d ↔ Cluster %d\n\n", section.bridge.ClusterA, section.bridge.ClusterB))
	sb.WriteString(fmt.Sprintf("Strength: %.2f\n\n", section.bridge.Strength))
	sb.WriteString("Cluster A:\n")
	sb.WriteString(fmt.Sprintf("- Label: %s\n", section.labelA))
	sb.WriteString("- Evidence:\n")
	if len(section.evidenceA) == 0 {
		sb.WriteString("  1. \"Insufficient evidence\"\n")
	} else {
		for i, snip := range section.evidenceA {
			sb.WriteString(fmt.Sprintf("  %d. \"%s\"\n", i+1, snip))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Cluster B:\n")
	sb.WriteString(fmt.Sprintf("- Label: %s\n", section.labelB))
	sb.WriteString("- Evidence:\n")
	if len(section.evidenceB) == 0 {
		sb.WriteString("  1. \"Insufficient evidence\"\n")
	} else {
		for i, snip := range section.evidenceB {
			sb.WriteString(fmt.Sprintf("  %d. \"%s\"\n", i+1, snip))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Shared Concept:\n")
	sb.WriteString(section.sharedConcept)
	sb.WriteString("\n\n")

	sb.WriteString("Analysis:\n")
	sb.WriteString(section.analysis)
	sb.WriteString("\n\n")

	sb.WriteString("Conclusion:\n")
	sb.WriteString(section.conclusion)
	sb.WriteString("\n\n")

	sb.WriteString("Review Flags:\n")
	for _, flag := range section.flags {
		sb.WriteString(fmt.Sprintf("- %s\n", flag))
	}
	sb.WriteString("\n")
	return sb.String()
}

func detectDuplicateHeavyCluster(cluster models.Cluster) bool {
	return cluster.Density >= 0.98 && cluster.Coherence >= 0.98
}

func calculateSourceBalance(cluster models.Cluster, metadataByID map[uint64]models.VectorMetadata) map[string]float64 {
	counts := map[string]int{}
	for _, id := range cluster.VectorIDs {
		meta, ok := metadataByID[id]
		if !ok {
			continue
		}
		source := meta.Source
		if source == "" {
			source = "unknown"
		}
		counts[source]++
	}
	if len(counts) == 0 {
		source := cluster.Source
		if source == "" {
			source = "unknown"
		}
		return map[string]float64{source: 100}
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	out := map[string]float64{}
	for source, count := range counts {
		out[source] = (float64(count) / float64(total)) * 100
	}
	return out
}

func detectSourceOversampling(sourceBalance map[string]float64) bool {
	for _, pct := range sourceBalance {
		if pct > 65 {
			return true
		}
	}
	return false
}

func cleanLLMOutput(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	cleaned := strings.ReplaceAll(text, "\r\n", "\n")
	leakPatterns := []string{
		"we need to analyze",
		"i'll produce",
		"maybe the user expects",
		"as an ai",
		"the prompt asks",
		"**thinking:**",
	}
	lines := strings.Split(cleaned, "\n")
	kept := make([]string, 0, len(lines))
	skipThinking := false
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "**thinking:") {
			skipThinking = true
			continue
		}
		if strings.HasPrefix(lower, "**conclusion:") {
			skipThinking = false
			continue
		}
		if skipThinking {
			continue
		}
		if containsLeakage(lower, leakPatterns) {
			continue
		}
		kept = append(kept, strings.TrimSpace(line))
	}
	joined := strings.TrimSpace(strings.Join(kept, "\n"))
	joined = strings.TrimSpace(strings.ReplaceAll(joined, "**Conclusion:**", ""))
	joined = strings.TrimSpace(strings.ReplaceAll(joined, "Conclusion:", ""))
	return joined
}

func containsLeakage(line string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

func validateInterpretationInputs(snippets []string) bool {
	return len(snippets) >= 3
}

func generateSuggestedLabel(cluster models.Cluster, snippets []string, finalConcept string) string {
	if finalConcept != "" && finalConcept != "Insufficient evidence" {
		label := sanitizeConceptLabel(finalConcept)
		if label != "" {
			return label
		}
	}
	if cluster.Label != "" {
		return sanitizeConceptLabel(cluster.Label)
	}
	if len(snippets) > 0 {
		return titleFromSnippet(snippets[0])
	}
	if cluster.Source != "" {
		return sanitizeConceptLabel(cluster.Source)
	}
	return fmt.Sprintf("Cluster %d", cluster.ID)
}

func renderSemanticAttractors(clusters []clusterSection, bridges []bridgeSection) []semanticAttractor {
	type observation struct {
		clusterID  int
		bridgeID   string
		confidence float64
		tokens     []string
	}
	observations := []observation{}
	addObs := func(name string, clusterID int, bridgeID string, confidence float64) {
		name = sanitizeConceptLabel(name)
		if name == "" {
			return
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "insufficient evidence") ||
			strings.Contains(lower, "pending interpretive") ||
			strings.Contains(lower, "shared concept unavailable") ||
			strings.Contains(lower, "no interpretive analysis") {
			return
		}
		toks := keywordTokens(name)
		if len(toks) == 0 {
			return
		}
		observations = append(observations, observation{
			clusterID: clusterID, bridgeID: bridgeID, confidence: confidence, tokens: toks,
		})
	}
	for _, c := range clusters {
		addObs(c.finalConcept, c.cluster.ID, "", c.confidence)
	}
	for _, b := range bridges {
		addObs(b.sharedConcept, 0, fmt.Sprintf("%d↔%d", b.bridge.ClusterA, b.bridge.ClusterB), b.confidence)
	}

	// Global frequency: number of distinct observations containing each token.
	tokenFreq := map[string]int{}
	for _, obs := range observations {
		seen := map[string]bool{}
		for _, tok := range obs.tokens {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			tokenFreq[tok]++
		}
	}

	type agg struct {
		token       string
		clusters    map[int]bool
		bridges     map[string]bool
		confTotal   float64
		count       int
	}
	groups := map[string]*agg{}
	for _, obs := range observations {
		// Anchor token = highest-frequency token in this observation (freq >= 2).
		anchor := ""
		bestFreq := 1
		for _, tok := range obs.tokens {
			if tokenFreq[tok] > bestFreq {
				bestFreq = tokenFreq[tok]
				anchor = tok
			}
		}
		if anchor == "" {
			continue
		}
		node := groups[anchor]
		if node == nil {
			node = &agg{token: anchor, clusters: map[int]bool{}, bridges: map[string]bool{}}
			groups[anchor] = node
		}
		if obs.clusterID > 0 {
			node.clusters[obs.clusterID] = true
		}
		if obs.bridgeID != "" {
			node.bridges[obs.bridgeID] = true
		}
		node.confTotal += obs.confidence
		node.count++
	}

	out := make([]semanticAttractor, 0, len(groups))
	for _, node := range groups {
		if len(node.clusters)+len(node.bridges) < 2 {
			continue
		}
		clusterIDs := make([]int, 0, len(node.clusters))
		for id := range node.clusters {
			clusterIDs = append(clusterIDs, id)
		}
		sort.Ints(clusterIDs)
		bridgeIDs := make([]string, 0, len(node.bridges))
		for id := range node.bridges {
			bridgeIDs = append(bridgeIDs, id)
		}
		sort.Strings(bridgeIDs)
		confidence := 0.0
		if node.count > 0 {
			confidence = node.confTotal / float64(node.count)
		}
		out = append(out, semanticAttractor{
			name:        attractorTitle(node.token),
			clusters:    clusterIDs,
			bridges:     bridgeIDs,
			confidence:  confidence,
			explanation: fmt.Sprintf("Keyword '%s' recurs across %d cluster(s) and %d bridge(s).", node.token, len(clusterIDs), len(bridgeIDs)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		si := len(out[i].clusters) + len(out[i].bridges)
		sj := len(out[j].clusters) + len(out[j].bridges)
		if si != sj {
			return si > sj
		}
		if out[i].confidence != out[j].confidence {
			return out[i].confidence > out[j].confidence
		}
		return out[i].name < out[j].name
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func attractorTitle(token string) string {
	if token == "" {
		return ""
	}
	return strings.ToUpper(token[:1]) + token[1:]
}

var attractorStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"these": true, "those": true, "from": true, "into": true, "within": true, "across": true,
	"about": true, "between": true, "through": true, "their": true, "there": true, "where": true,
	"which": true, "while": true, "such": true, "some": true, "more": true, "less": true,
	"also": true, "than": true, "then": true, "they": true, "them": true, "your": true,
	"will": true, "would": true, "could": true, "should": true, "must": true, "have": true,
	"having": true, "been": true, "being": true, "what": true, "when": true, "very": true,
	"each": true, "other": true, "another": true, "concept": true, "concepts": true,
	"cluster": true, "clusters": true, "bridge": true, "bridges": true, "shared": true,
	"semantic": true, "text": true, "passage": true, "passages": true, "vector": true,
	"vectors": true, "embodies": true, "represents": true, "represent": true, "captures": true,
	"captured": true, "describe": true, "describes": true, "described": true, "discusses": true,
	"discussed": true, "highlights": true, "highlighted": true, "explore": true, "explores": true,
	"explored": true, "focus": true, "focused": true, "focuses": true, "centers": true,
	"centered": true, "centering": true, "involves": true, "involve": true, "involved": true,
	"various": true, "differ": true, "different": true, "specifically": true, "particularly": true,
	"include": true, "includes": true, "including": true, "based": true,
	"primarily": true, "essentially": true, "overall": true, "thus": true, "therefore": true,
	"hence": true, "however": true, "moreover": true, "although": true, "though": true,
	"because": true, "since": true, "after": true, "before": true, "during": true,
	"only": true, "just": true, "still": true, "even": true, "much": true,
	"high": true, "density": true, "coherence": true, "material": true,
}

func keywordTokens(text string) []string {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	re := regexp.MustCompile(`[a-z][a-z0-9_'-]+`)
	raw := re.FindAllString(lower, -1)
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, tok := range raw {
		tok = strings.Trim(tok, "-'")
		if len(tok) < 4 {
			continue
		}
		if attractorStopwords[tok] {
			continue
		}
		if seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

func buildRecommendations(clusters []clusterSection, bridges []bridgeSection) []string {
	recs := []string{
		"Remove or deduplicate near-identical chunks before the next run.",
		"Rebalance overrepresented sources in clusters with oversampling warnings.",
		"Add more diverse source material for weakly evidenced clusters.",
		"Re-run clustering after data cleanup to verify topology stability.",
		"Require representative snippets on both bridge sides before interpretation.",
		"Review low-confidence taxonomy classifications and label mismatches.",
	}
	if hasSkippedBridges(bridges) {
		recs = append(recs, "Backfill missing bridge snippets from mb_ collections before semantic interpretation.")
	}
	if hasDuplicateHeavy(clusters) {
		recs = append(recs, "Prioritize manual review of duplicate-heavy clusters marked as low-trust.")
	}
	return recs
}

func hasSkippedBridges(bridges []bridgeSection) bool {
	for _, b := range bridges {
		if b.skipped {
			return true
		}
	}
	return false
}

func hasDuplicateHeavy(clusters []clusterSection) bool {
	for _, c := range clusters {
		if c.duplicateHeavy {
			return true
		}
	}
	return false
}

func summarizeDiagnostics(clusters []clusterSection, bridges []bridgeSection) reportDiagnostics {
	out := reportDiagnostics{}
	for _, c := range clusters {
		if c.duplicateHeavy {
			out.duplicateHeavyClusters++
		}
		if c.overSampled {
			out.overSampledClusters++
		}
	}
	for _, b := range bridges {
		if b.skipped {
			out.skippedBridges++
		}
	}
	return out
}

func recommendedReviewItems(clusters []clusterSection, bridges []bridgeSection) []string {
	items := []string{}
	for _, c := range clusters {
		if c.duplicateHeavy {
			items = append(items, fmt.Sprintf("Cluster %d duplicate-heavy: validate deduplication", c.cluster.ID))
		}
		if c.overSampled {
			items = append(items, fmt.Sprintf("Cluster %d source oversampling exceeds 65%%", c.cluster.ID))
		}
	}
	for _, b := range bridges {
		if b.skipped {
			items = append(items, fmt.Sprintf("Bridge %d↔%d skipped: insufficient evidence", b.bridge.ClusterA, b.bridge.ClusterB))
		}
	}
	return dedupeStrings(items)
}

func bridgeEvidence(bridge models.Bridge, metadataByID map[uint64]models.VectorMetadata) ([]string, []string) {
	left := []string{}
	right := []string{}
	seenLeft := map[string]bool{}
	seenRight := map[string]bool{}
	for _, sample := range bridge.SampleLinks {
		if meta, ok := metadataByID[sample.ChunkAID]; ok && meta.Fragment != "" && meta.Fragment != "N/A" {
			snip := truncate(meta.Fragment, 240)
			if !seenLeft[snip] {
				left = append(left, snip)
				seenLeft[snip] = true
			}
		}
		if meta, ok := metadataByID[sample.ChunkBID]; ok && meta.Fragment != "" && meta.Fragment != "N/A" {
			snip := truncate(meta.Fragment, 240)
			if !seenRight[snip] {
				right = append(right, snip)
				seenRight[snip] = true
			}
		}
		if len(left) >= 2 && len(right) >= 2 {
			break
		}
	}
	return left, right
}

func representativeSnippets(cluster models.Cluster, metadataByID map[uint64]models.VectorMetadata, max int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, id := range cluster.VectorIDs {
		meta, ok := metadataByID[id]
		if !ok || meta.Fragment == "" || meta.Fragment == "N/A" {
			continue
		}
		snip := truncate(meta.Fragment, 240)
		if seen[snip] {
			continue
		}
		seen[snip] = true
		out = append(out, snip)
		if len(out) >= max {
			break
		}
	}
	return out
}

func extractTopologyDiagnostics(findings []models.Finding) topologyDiagnostics {
	for _, finding := range findings {
		if finding.Type == "topology_diagnostics" {
			return topologyDiagnostics{subject: finding.Subject, reason: finding.ReasoningChain}
		}
	}
	return topologyDiagnostics{}
}

func clusterFindingsByID(findings []models.Finding) map[int]models.Finding {
	out := map[int]models.Finding{}
	for _, finding := range findings {
		if finding.Type != "cluster_analysis" {
			continue
		}
		id := 0
		if len(finding.Clusters) > 0 {
			id = finding.Clusters[0]
		} else {
			fmt.Sscanf(finding.Subject, "Cluster %d", &id)
		}
		if id > 0 {
			out[id] = finding
		}
	}
	return out
}

func bridgeFindingsByPair(findings []models.Finding) map[string]models.Finding {
	out := map[string]models.Finding{}
	for _, finding := range findings {
		if finding.Type != "bridge_analysis" {
			continue
		}
		a, b := 0, 0
		if len(finding.Clusters) >= 2 {
			a, b = finding.Clusters[0], finding.Clusters[1]
		} else {
			fmt.Sscanf(finding.Subject, "Bridge: %d ↔ %d", &a, &b)
		}
		if a > 0 && b > 0 {
			out[bridgeKey(a, b)] = finding
			out[bridgeKey(b, a)] = finding
		}
	}
	return out
}

func anomalyFlagsByCluster(findings []models.Finding) map[int][]string {
	out := map[int][]string{}
	for _, finding := range findings {
		if !finding.IsAnomaly {
			continue
		}
		for _, clusterID := range finding.Clusters {
			label := finding.AnomalyType
			if label == "" {
				label = finding.Type
			}
			if finding.ConfidenceBand != "" {
				label = fmt.Sprintf("%s (%s %.2f)", label, finding.ConfidenceBand, finding.Confidence)
			}
			out[clusterID] = append(out[clusterID], label)
		}
	}
	for key := range out {
		out[key] = dedupeStrings(out[key])
	}
	return out
}

func metadataByID(metadata []models.VectorMetadata) map[uint64]models.VectorMetadata {
	out := make(map[uint64]models.VectorMetadata, len(metadata))
	for _, item := range metadata {
		out[item.ID] = item
	}
	return out
}

func clusterByID(clusters []models.Cluster) map[int]models.Cluster {
	out := make(map[int]models.Cluster, len(clusters))
	for _, cluster := range clusters {
		out[cluster.ID] = cluster
	}
	return out
}

func bridgeKey(a, b int) string {
	return fmt.Sprintf("%d:%d", a, b)
}

func sourceBalanceLines(balance map[string]float64) []string {
	type row struct {
		source string
		pct    float64
	}
	rows := make([]row, 0, len(balance))
	for source, pct := range balance {
		rows = append(rows, row{source: source, pct: pct})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].pct == rows[j].pct {
			return rows[i].source < rows[j].source
		}
		return rows[i].pct > rows[j].pct
	})
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.pct < 1.0 {
			continue // suppress sources contributing <1% — noise in the report
		}
		lines = append(lines, fmt.Sprintf("- %s: %.0f%%", row.source, row.pct))
	}
	return lines
}

func taxonomyFields(cluster models.Cluster) (string, string, string) {
	if cluster.Taxonomy == nil {
		return "unknown", "unknown", "unknown"
	}
	return fallback(cluster.Taxonomy.Topic, "unknown"), fallback(cluster.Taxonomy.Mode, "unknown"), fallback(cluster.Taxonomy.EpistemicPosture, "unknown")
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

func clusterConfidence(cluster models.Cluster, finding models.Finding) float64 {
	if cluster.Taxonomy != nil && cluster.Taxonomy.Confidence > 0 {
		return cluster.Taxonomy.Confidence
	}
	if finding.Confidence > 0 {
		return finding.Confidence
	}
	return 0.5
}

func bridgeConfidence(bridge models.Bridge, finding models.Finding) float64 {
	if finding.Confidence > 0 {
		return finding.Confidence
	}
	return bridge.Strength
}

func bridgeSharedConcept(bridge models.Bridge, analysis string) string {
	if bridge.Label != "" {
		return sanitizeConceptLabel(bridge.Label)
	}
	if strings.TrimSpace(analysis) == "" {
		return "Shared concept unavailable"
	}
	concept := extractFinalConcept(analysis)
	if concept != "" {
		return concept
	}
	return "Shared concept unavailable"
}

func collapseConclusionBlocks(text string) string {
	re := regexp.MustCompile(`(?is)\*\*conclusion:\*\*`)
	parts := re.Split(text, -1)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func extractFinalConcept(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		last = strings.TrimSpace(trimmed)
	}
	return sanitizeConceptLabel(last)
}

func sanitizeConceptLabel(text string) string {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return ""
	}
	prefixes := []string{
		"the cluster embodies the concept of",
		"the semantic concept is",
		"the concept is",
		"thus, the cluster represents",
		"the cluster captures the semantic concept of",
		"the cluster captures the core concept of",
		"the cluster captures",
		"this cluster represents",
		"this cluster captures",
		"this cluster centers on",
		"this cluster describes",
		"this cluster consists of",
		"this cluster consists entirely of",
		"this cluster consists",
		"this cluster contains",
		"the bridging concept is",
		"the bridge concept is",
		"the shared concept bridging these clusters is",
		"the shared concept is",
		"shared concept:",
		"the cluster represents",
		"conclusion:",
	}
	for {
		lower := strings.ToLower(cleaned)
		stripped := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(lower, prefix) {
				cleaned = strings.TrimSpace(cleaned[len(prefix):])
				cleaned = strings.TrimLeft(cleaned, ":,;.* \t")
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}
	// Meta-introducers ("The cluster's label X indicates …", "The concept represented by this cluster is …")
	// don't strip cleanly as prefixes. Detect them and cut at the connecting verb.
	cleaned = stripMetaIntroducer(cleaned)
	cleaned = strings.Trim(cleaned, "\"'`* .")
	return cleaned
}

// stripMetaIntroducer removes meta-language openers like "The cluster's label X indicates Y"
// by detecting an opener and cutting at the first connecting verb, returning what follows.
func stripMetaIntroducer(text string) string {
	lower := strings.ToLower(text)
	openers := []string{
		"the cluster's label",
		"the cluster label",
		"the concept represented by this cluster is",
		"the concept of this cluster is",
		"the concept represented here is",
		"the cluster consists",
		"the cluster contains",
		"the cluster is",
		"the cluster centers on",
		"the cluster captures",
		"the cluster describes",
		"the cluster represents",
		"this cluster's label",
		"the passage",
		"the text",
	}
	hasOpener := false
	for _, o := range openers {
		if strings.HasPrefix(lower, o) {
			hasOpener = true
			break
		}
	}
	if !hasOpener {
		return text
	}
	cuts := []string{
		" indicates that ", " indicates ",
		" represents that ", " represents ",
		" describes that ", " describes ",
		" describing ",
		" centers on ", " centered on ",
		" captures the ", " captures ",
		" consists of ", " consists entirely of ",
		" contains ",
		" is **", " is ",
	}
	bestIdx := -1
	bestCut := ""
	for _, c := range cuts {
		idx := strings.Index(lower, c)
		if idx < 0 {
			continue
		}
		// Prefer the earliest meaningful cut to avoid skipping past clauses.
		if bestIdx == -1 || idx < bestIdx {
			bestIdx = idx
			bestCut = c
		}
	}
	if bestIdx < 0 {
		return text
	}
	return strings.TrimLeft(text[bestIdx+len(bestCut):], "* \t")
}

// stripTrailingEcho removes the last sentence/line of analysis if it duplicates the
// conclusion (or shares heavy keyword overlap with it), reducing visual redundancy.
func stripTrailingEcho(analysis, conclusion string) string {
	analysis = strings.TrimSpace(analysis)
	conclusion = strings.TrimSpace(conclusion)
	if analysis == "" || conclusion == "" {
		return analysis
	}
	sentences := splitSentences(analysis)
	if len(sentences) < 2 {
		return analysis
	}
	last := strings.TrimSpace(sentences[len(sentences)-1])
	if isEchoOf(last, conclusion) {
		kept := strings.Join(sentences[:len(sentences)-1], " ")
		return strings.TrimSpace(kept)
	}
	return analysis
}

func isEchoOf(sentence, conclusion string) bool {
	s := strings.ToLower(strings.Trim(sentence, ".\"'`* "))
	c := strings.ToLower(strings.Trim(conclusion, ".\"'`* "))
	if s == "" || c == "" {
		return false
	}
	if s == c {
		return true
	}
	if strings.Contains(s, c) || strings.Contains(c, s) {
		return true
	}
	// Token-overlap heuristic.
	sToks := keywordTokens(s)
	cToks := keywordTokens(c)
	if len(cToks) < 2 {
		return false
	}
	cSet := make(map[string]bool, len(cToks))
	for _, t := range cToks {
		cSet[t] = true
	}
	overlap := 0
	for _, t := range sToks {
		if cSet[t] {
			overlap++
		}
	}
	// If ≥75% of conclusion's keywords appear in the trailing sentence, treat as echo.
	return float64(overlap)/float64(len(cToks)) >= 0.75
}

// shortenForHeading truncates for headings without ending mid-word or mid-sentence.
func shortenForHeading(text string, max int) string {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return ""
	}
	if len(cleaned) <= max {
		return cleaned
	}

	// Prefer a full sentence boundary at or near max.
	punct := ".!?"
	if idx := strings.LastIndexAny(cleaned[:max], punct); idx > 20 {
		return strings.TrimSpace(cleaned[:idx+1])
	}
	for i := max; i < len(cleaned) && i < max+80; i++ {
		if strings.ContainsRune(punct, rune(cleaned[i])) {
			return strings.TrimSpace(cleaned[:i+1])
		}
	}

	// No sentence boundary nearby: return unchanged rather than clipping mid-sentence.
	return cleaned
}

func titleFromSnippet(snippet string) string {
	words := strings.Fields(snippet)
	if len(words) == 0 {
		return "Evidence-based cluster"
	}
	if len(words) > 7 {
		words = words[:7]
	}
	text := strings.Join(words, " ")
	return strings.Trim(text, ".,;:!?\"'")
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func limitSentences(text string, max int) string {
	parts := splitSentences(text)
	if len(parts) == 0 {
		return text
	}
	if len(parts) > max {
		parts = parts[:max]
	}
	return strings.Join(parts, " ")
}

func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	re := regexp.MustCompile(`[^.!?]+[.!?]`)
	parts := re.FindAllString(text, -1)
	if len(parts) == 0 {
		return []string{text}
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func dedupeStrings(input []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, item := range input {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ", ")
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
