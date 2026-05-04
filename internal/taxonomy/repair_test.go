package taxonomy

import (
	"strings"
	"testing"

	"github.com/meistro57/vectoreologist/internal/models"
)

func TestCheckLabelMismatch_QuantumLabelConsciousnessContent(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "consciousness_philosophy", Mode: ModeScholarlyAnnotation}
	warn := CheckLabelMismatch(lbl, "other / quantum_mechanics")
	if warn == "" {
		t.Error("expected a warning when label says 'quantum' but content is consciousness_philosophy")
	}
	if !strings.Contains(warn, "quantum") {
		t.Errorf("warning should mention 'quantum', got: %q", warn)
	}
}

func TestCheckLabelMismatch_DidacticModeConflict(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "general", Mode: ModeScholarlyAnnotation}
	warn := CheckLabelMismatch(lbl, "didactic / General Content")
	if warn == "" {
		t.Error("expected warning when label says 'didactic' but mode is scholarly_annotation")
	}
}

func TestCheckLabelMismatch_NoMismatch_MatchingTopic(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "quantum_mechanics", Mode: ModeScholarlyAnnotation}
	warn := CheckLabelMismatch(lbl, "other / quantum_mechanics")
	if warn != "" {
		t.Errorf("no warning expected when label and classifier agree on quantum, got: %q", warn)
	}
}

func TestCheckLabelMismatch_NilLabel(t *testing.T) {
	warn := CheckLabelMismatch(nil, "anything")
	if warn != "" {
		t.Errorf("nil TaxonomyLabel should return empty warning, got: %q", warn)
	}
}

func TestCheckLabelMismatch_GeneralTopicNoWarning(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "general", Mode: ModeUnknown}
	warn := CheckLabelMismatch(lbl, "other / quantum_mechanics")
	// "general" topic can't definitively determine a mismatch.
	if warn != "" {
		t.Errorf("general topic should not trigger mismatch warning, got: %q", warn)
	}
}

func TestCheckLabelMismatch_ConsciousnessLabelMatchesContent(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "consciousness_philosophy", Mode: ModeScholarlyAnnotation}
	warn := CheckLabelMismatch(lbl, "surface / consciousness exploration")
	if warn != "" {
		t.Errorf("no warning expected when label contains 'consciousness' and topic is consciousness_philosophy, got: %q", warn)
	}
}

func TestCheckLabelMismatch_HistoryLabelPhilosophyContent(t *testing.T) {
	lbl := &models.TaxonomyLabel{Topic: "philosophy", Mode: ModeDidacticTeaching}
	warn := CheckLabelMismatch(lbl, "deep / ancient history")
	if warn == "" {
		t.Error("expected warning when label says 'history' but content is philosophy")
	}
}
