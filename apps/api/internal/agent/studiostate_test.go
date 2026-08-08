package agent

import (
	"encoding/json"
	"testing"
)

func TestDefaultStudioStateRoundTrips(t *testing.T) {
	def := DefaultStudioState()
	if def.Stage != StageTopicDiscussion || def.OpenTool != ToolChat || def.WidthTier != WidthChat {
		t.Fatalf("unexpected default: %+v", def)
	}
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	var got StudioState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Stage != def.Stage || got.OpenTool != def.OpenTool || got.Reference == nil {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestWidthForTool(t *testing.T) {
	cases := map[OpenTool]WidthTier{
		ToolChat: WidthChat, ToolForming: WidthHalf, ToolPlan: WidthWide,
		ToolReading: WidthWide, ToolWriting: WidthWide, ToolReflection: WidthWide,
	}
	for tool, want := range cases {
		if got := WidthForTool(tool); got != want {
			t.Errorf("WidthForTool(%q)=%q want %q", tool, got, want)
		}
	}
}

func TestWidthForTool_FormingAndPlan(t *testing.T) {
	if WidthForTool(ToolForming) != WidthHalf {
		t.Errorf("forming → %s, want half", WidthForTool(ToolForming))
	}
	if WidthForTool(ToolPlan) != WidthWide {
		t.Errorf("plan → %s, want wide", WidthForTool(ToolPlan))
	}
	if !ToolForming.IsValid() {
		t.Error("forming must be valid")
	}
}

func TestStageIsValid(t *testing.T) {
	if !StageBodyWriting.IsValid() || StudioStage("nope").IsValid() {
		t.Fatal("IsValid wrong")
	}
}

// TestStudioStateStartedDefault covers the `started` flag added alongside
// the studio-onboarding start gate: brand-new state must default to false,
// and a legacy studio_state jsonb blob (persisted before this field
// existed, so it has no "started" key) must unmarshal to false too — this
// pins the contract so projects mid-journey before migration 0059's
// backfill runs still read as "not started" rather than panicking or
// zero-valuing into something else.
func TestStudioStateStartedDefault(t *testing.T) {
	if DefaultStudioState().Started {
		t.Fatal("default Started must be false")
	}

	// legacy jsonb without the key → Started false
	var s StudioState
	legacy := `{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}`
	if err := json.Unmarshal([]byte(legacy), &s); err != nil {
		t.Fatal(err)
	}
	if s.Started {
		t.Fatal("legacy state must unmarshal Started=false")
	}
}
