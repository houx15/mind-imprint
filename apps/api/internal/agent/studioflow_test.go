package agent

import (
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
)

func TestStatusRegistry_Integrity(t *testing.T) {
	reg := StatusRegistry()
	for _, f := range []FlowStatus{FlowTopic, FlowFramework, FlowProposal, FlowEssay, FlowReview} {
		def, ok := reg[f]
		if !ok {
			t.Fatalf("status %q missing from registry", f)
		}
		if !def.Surface.IsValid() {
			t.Errorf("status %q has invalid surface %q", f, def.Surface)
		}
		if strings.TrimSpace(def.SystemPrompt) == "" || strings.TrimSpace(def.Goal) == "" {
			t.Errorf("status %q has empty goal/prompt", f)
		}
		for _, tool := range def.Tools {
			if !IsKnownStatusTool(tool) {
				t.Errorf("status %q lists unknown tool %q", f, tool)
			}
		}
		for _, cardID := range def.Cards {
			if _, found := cards.ByID(cardID); !found {
				t.Errorf("status %q references unknown card %q", f, cardID)
			}
		}
	}
	// Doc mapping per spec.
	if reg[FlowFramework].Doc != DocNone {
		t.Errorf("framework doc should be none, got %q", reg[FlowFramework].Doc)
	}
	if reg[FlowProposal].Doc != DocProposal {
		t.Errorf("proposal doc should be proposal, got %q", reg[FlowProposal].Doc)
	}
	if reg[FlowEssay].Doc != DocEssay {
		t.Errorf("essay doc should be essay, got %q", reg[FlowEssay].Doc)
	}
	// review has no tools (supportive reflection only).
	if len(reg[FlowReview].Tools) != 0 {
		t.Errorf("review should have no tools, got %v", reg[FlowReview].Tools)
	}
}

func TestStatusForStage_Collapse(t *testing.T) {
	cases := map[StudioStage]FlowStatus{
		StageTopicDiscussion: FlowTopic,
		StageProposalForming: FlowFramework,
		StagePlanGeneration:  FlowFramework,
		StageProposalWriting: FlowProposal,
		StageProposalReview:  FlowProposal,
		StageBodyWriting:     FlowEssay,
		StageRetrospective:   FlowReview,
	}
	for stage, want := range cases {
		if got := StatusForStage(stage); got != want {
			t.Errorf("StatusForStage(%q) = %q, want %q", stage, got, want)
		}
	}
	// Unknown / empty → framework (safe working default, never a dead end).
	if got := StatusForStage(StudioStage("nonsense")); got != FlowFramework {
		t.Errorf("unknown stage should collapse to framework, got %q", got)
	}
	if got := StatusForStage(""); got != FlowFramework {
		t.Errorf("empty stage should collapse to framework, got %q", got)
	}
}

func TestStageFloor_RoundTripsWithinStatus(t *testing.T) {
	// A status' floor must itself collapse back to that status (idempotent view).
	for _, f := range []FlowStatus{FlowTopic, FlowFramework, FlowProposal, FlowEssay, FlowReview} {
		if got := StatusForStage(f.StageFloor()); got != f {
			t.Errorf("StageFloor round-trip: %q → stage %q → %q", f, f.StageFloor(), got)
		}
	}
}
