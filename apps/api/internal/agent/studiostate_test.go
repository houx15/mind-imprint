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

func TestStudioState_ProposalTrackRoundTrip(t *testing.T) {
	in := DefaultStudioState()
	in.ProposalTrack = &WritingTrack{
		Mode:         ModeGuided,
		Started:      true,
		StepIndex:    2,
		SubQuestions: []SubQuestion{{ID: "a", Text: "q1"}},
		StepGuides:   map[string]string{"understanding": "{}"},
	}
	in.CounterpointsWaived = true

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out StudioState
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ProposalTrack == nil {
		t.Fatal("proposalTrack lost in round-trip")
	}
	if out.ProposalTrack.Mode != ModeGuided || out.ProposalTrack.StepIndex != 2 {
		t.Fatalf("track fields wrong: %+v", out.ProposalTrack)
	}
	if len(out.ProposalTrack.SubQuestions) != 1 || out.ProposalTrack.SubQuestions[0].ID != "a" {
		t.Fatalf("sub-questions lost: %+v", out.ProposalTrack.SubQuestions)
	}
	if !out.CounterpointsWaived {
		t.Fatal("counterpointsWaived lost in round-trip")
	}
}

func TestDefaultStudioState_NoTrack(t *testing.T) {
	s := DefaultStudioState()
	if s.ProposalTrack != nil {
		t.Fatalf("default should have nil proposalTrack, got %+v", s.ProposalTrack)
	}
	if s.CounterpointsWaived {
		t.Fatal("default should have counterpointsWaived=false")
	}
	if s.EssayTrack != nil {
		t.Fatalf("default should have nil essayTrack, got %+v", s.EssayTrack)
	}
}

func TestStudioState_EssayTrackRoundTrip(t *testing.T) {
	in := DefaultStudioState()
	in.EssayTrack = &EssayTrack{Stage: EssayResearch}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out StudioState
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.EssayTrack == nil || out.EssayTrack.Stage != EssayResearch {
		t.Fatalf("essayTrack lost: %+v", out.EssayTrack)
	}
}
