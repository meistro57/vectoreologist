package lens

import (
	"testing"

	"github.com/meistro57/vectoreologist/internal/synthesis"
)

func TestApplyFilters_SortOrder(t *testing.T) {
	t.Parallel()

	baseClusters := []synthesis.JSONCluster{
		{ID: 3, Coherence: 0.4, Density: 0.9, Size: 12},
		{ID: 1, Coherence: 0.9, Density: 0.2, Size: 3},
		{ID: 2, Coherence: 0.7, Density: 0.5, Size: 8},
	}

	tests := []struct {
		name      string
		sortField SortField
		sortAsc   bool
		wantIDs   []int
	}{
		{name: "coherence descending", sortField: sortByCoherence, sortAsc: false, wantIDs: []int{1, 2, 3}},
		{name: "density ascending", sortField: sortByDensity, sortAsc: true, wantIDs: []int{1, 2, 3}},
		{name: "size descending", sortField: sortBySize, sortAsc: false, wantIDs: []int{3, 2, 1}},
		{name: "id ascending", sortField: sortByID, sortAsc: true, wantIDs: []int{1, 2, 3}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := Model{
				report:    &synthesis.JSONReport{Clusters: baseClusters},
				sortField: tc.sortField,
				sortAsc:   tc.sortAsc,
			}
			m.applyFilters()

			if len(m.visibleClusters) != len(tc.wantIDs) {
				t.Fatalf("visible cluster count = %d, want %d", len(m.visibleClusters), len(tc.wantIDs))
			}
			for i, wantID := range tc.wantIDs {
				if m.visibleClusters[i].ID != wantID {
					t.Fatalf("index %d ID = %d, want %d (order=%+v)", i, m.visibleClusters[i].ID, wantID, m.visibleClusters)
				}
			}
		})
	}
}
