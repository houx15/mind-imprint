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

func TestStatusRegistry_FinalizedDecks(t *testing.T) {
	reg := StatusRegistry()
	want := map[FlowStatus][]string{
		FlowFramework: nil, // 提问卡 moved to a chatbox button — no longer an AI-summonable deck card
		FlowProposal:  nil,
		FlowEssay:     {"pee", "toulmin", "argument-map"},
		FlowReview:    nil,
	}
	for st, exp := range want {
		got := reg[st].Cards
		if len(got) != len(exp) {
			t.Errorf("%s deck = %v, want %v", st, got, exp)
			continue
		}
		for i := range exp {
			if got[i] != exp[i] {
				t.Errorf("%s deck = %v, want %v", st, got, exp)
				break
			}
		}
	}
	// search-plan is AI-side (reading) — never in a writing-flow deck.
	for st, def := range reg {
		for _, id := range def.Cards {
			if id == "search-plan" {
				t.Errorf("search-plan must not be in the %s deck", st)
			}
		}
	}
}

func TestIsSummonable(t *testing.T) {
	// cross-cutting cards summon in ANY status.
	for _, st := range []FlowStatus{FlowFramework, FlowProposal, FlowEssay} {
		if !IsSummonable("ai-boundary", st) {
			t.Errorf("cross-cutting ai-boundary should be summonable in %s", st)
		}
	}
	if !IsSummonable("perspective-matrix", FlowFramework) {
		t.Error("perspective-matrix (cross-cutting) should be summonable in framework")
	}
	if !IsSummonable("concession", FlowEssay) {
		t.Error("concession (cross-cutting) should be summonable in essay")
	}
	// status-deck cards summon in their status.
	if !IsSummonable("toulmin", FlowEssay) {
		t.Error("toulmin should be summonable in essay")
	}
	// question-card is NOT a coach-summonable card anymore — it's a chatbox
	// button that opens the modal directly (student-opened, not AI-proposed).
	if IsSummonable("question-card", FlowFramework) {
		t.Error("question-card must NOT be summonable — it's a chatbox button now")
	}
	// reading-toolkit cards are NOT summonable via writing-flow statuses.
	if IsSummonable("cda", FlowEssay) {
		t.Error("cda (reading-toolkit) must NOT be summonable in essay")
	}
	if IsSummonable("search-plan", FlowFramework) {
		t.Error("search-plan (reading-toolkit/AI-side) must NOT be summonable in framework")
	}
}

func TestPlacementTagsMatchBindingLists(t *testing.T) {
	// Build the authoritative placement from the Go binding lists...
	expected := map[string]string{}
	reg := StatusRegistry()
	for _, st := range []FlowStatus{FlowTopic, FlowFramework, FlowProposal, FlowEssay, FlowReview} {
		for _, id := range reg[st].Cards {
			expected[id] = "status"
		}
	}
	expected["learning-report"] = "status" // function producer on the review surface (not in a deck)
	expected["question-card"] = "status"   // 提问卡: framework-phase card opened by a chatbox button (not an AI-summonable deck card)
	for _, id := range ReadingDeckIDs {
		expected[id] = "reading"
	}
	for _, id := range ReadingToolkitIDs {
		expected[id] = "reading-toolkit"
	}
	for _, id := range CrossCuttingCardIDs {
		expected[id] = "cross-cutting"
	}

	// ...and assert every card's JSON placement tag agrees with it.
	cat, err := cards.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cat {
		want, ok := expected[s.ID]
		if !ok {
			t.Errorf("card %q is unplaced — in no binding list", s.ID)
			continue
		}
		if s.Placement != want {
			t.Errorf("card %q placement tag = %q, want %q", s.ID, s.Placement, want)
		}
	}
	if len(expected) != len(cat) {
		t.Errorf("binding lists cover %d ids, registry has %d cards", len(expected), len(cat))
	}
	for id := range expected {
		if _, ok := cards.ByID(id); !ok {
			t.Errorf("binding lists reference unknown card %q", id)
		}
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
