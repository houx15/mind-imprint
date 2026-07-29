package agent

import (
	"strings"
	"testing"
)

// TestBuildProjectCoachContext — the one agent's user message names the active
// room, lays out the whole (non-folded) conversation oldest→newest with mapped
// speakers, and carries the spine projection as reference.
func TestBuildProjectCoachContext(t *testing.T) {
	history := []ChatTurn{
		{Role: "user", Content: "我想聊聊这个题目"},
		{Role: "assistant", Content: "你最想弄清楚的是哪一点？"},
		{Role: "user", Content: "中国的碳排放算不算反例"},
	}
	got := BuildProjectCoachContext(history, "开题四问：（还没落定）", "写作")

	for _, want := range []string{
		"学生现在在「写作」",
		"- 学生：我想聊聊这个题目",
		"- 你：你最想弄清楚的是哪一点？",
		"- 学生：中国的碳排放算不算反例",
		"项目当前状态",
		"开题四问：（还没落定）",
		"回应学生最新的发言",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context missing %q:\n%s", want, got)
		}
	}
}

// TestBuildProjectCoachContext_NoSurfaceNoProjection — both optional blocks are
// omitted cleanly when empty (a fresh project, unknown room).
func TestBuildProjectCoachContext_NoSurfaceNoProjection(t *testing.T) {
	got := BuildProjectCoachContext([]ChatTurn{{Role: "user", Content: "在吗"}}, "", "")
	if strings.Contains(got, "学生现在在") {
		t.Fatalf("empty surface should omit the room line:\n%s", got)
	}
	if strings.Contains(got, "项目当前状态") {
		t.Fatalf("empty projection should omit the state block:\n%s", got)
	}
	if !strings.Contains(got, "- 学生：在吗") {
		t.Fatalf("history line missing:\n%s", got)
	}
}
