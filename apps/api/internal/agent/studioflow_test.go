package agent

import "testing"

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
