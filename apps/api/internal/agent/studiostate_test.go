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
