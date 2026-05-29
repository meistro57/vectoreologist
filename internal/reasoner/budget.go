package reasoner

import (
	"sort"
	"strings"

	"github.com/meistro57/vectoreologist/internal/models"
)

type ReasoningBudget struct {
	MaxClusters int
	MaxBridges  int
	MaxMoats    int
}

func BudgetForProfile(profile string) ReasoningBudget {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "fast":
		return ReasoningBudget{MaxClusters: 12, MaxBridges: 4, MaxMoats: 2}
	case "deep":
		return ReasoningBudget{MaxClusters: 0, MaxBridges: 25, MaxMoats: 12}
	default:
		return ReasoningBudget{MaxClusters: 0, MaxBridges: 10, MaxMoats: 5}
	}
}

func ResolveBudget(profile string, maxClusters, maxBridges, maxMoats int) (ReasoningBudget, string) {
	resolvedProfile := strings.ToLower(strings.TrimSpace(profile))
	if resolvedProfile == "" {
		resolvedProfile = "balanced"
	}
	if resolvedProfile != "fast" && resolvedProfile != "balanced" && resolvedProfile != "deep" {
		resolvedProfile = "balanced"
	}
	budget := BudgetForProfile(resolvedProfile)
	if maxClusters >= 0 {
		budget.MaxClusters = maxClusters
	}
	if maxBridges >= 0 {
		budget.MaxBridges = maxBridges
	}
	if maxMoats >= 0 {
		budget.MaxMoats = maxMoats
	}
	return budget, resolvedProfile
}

func (r *Reasoner) SetBudget(budget ReasoningBudget) {
	r.budget = budget
}

func (r *Reasoner) SetFindingHandler(handler func(models.Finding, int, int)) {
	r.findingHandler = handler
}

func applyClusterBudget(clusters []models.Cluster, max int) []models.Cluster {
	selected := append([]models.Cluster(nil), clusters...)
	if max <= 0 || len(selected) <= max {
		return selected
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Size != selected[j].Size {
			return selected[i].Size > selected[j].Size
		}
		if selected[i].Coherence != selected[j].Coherence {
			return selected[i].Coherence > selected[j].Coherence
		}
		return selected[i].ID < selected[j].ID
	})
	selected = selected[:max]
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].ID < selected[j].ID
	})
	return selected
}

func applyBridgeBudget(bridges []models.Bridge, max int) []models.Bridge {
	selected := append([]models.Bridge(nil), bridges...)
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Strength != selected[j].Strength {
			return selected[i].Strength > selected[j].Strength
		}
		if selected[i].ClusterA != selected[j].ClusterA {
			return selected[i].ClusterA < selected[j].ClusterA
		}
		return selected[i].ClusterB < selected[j].ClusterB
	})
	if max > 0 && len(selected) > max {
		selected = selected[:max]
	}
	return selected
}

func applyMoatBudget(moats []models.Moat, max int) []models.Moat {
	selected := append([]models.Moat(nil), moats...)
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Distance != selected[j].Distance {
			return selected[i].Distance > selected[j].Distance
		}
		if selected[i].ClusterA != selected[j].ClusterA {
			return selected[i].ClusterA < selected[j].ClusterA
		}
		return selected[i].ClusterB < selected[j].ClusterB
	})
	if max > 0 && len(selected) > max {
		selected = selected[:max]
	}
	return selected
}
