package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

func TestBuildCourseContextIsTheCourseRecipe(t *testing.T) {
	script := CourseScript{
		CourseTitle: "一条网络信息，该不该信",
		PhaseTitles: []string{"演示", "引导", "独立", "回看"},
		CurrentIdx:  1,
	}
	phase := skills.Contract{
		Goal:          "在这条真实说法上，带学生做一次完整的信源辨识",
		SoftCondition: "学生对这条说法做完了一次真实的溯源，不是走过场",
	}
	history := []ChatTurn{{Role: "student", Content: "这条是真的吧？"}, {Role: "assistant", Content: "你怎么看？"}}

	got := BuildCourseContext(script, phase, history, "CRAAP 卡：进行中", "advance")

	for _, want := range []string{
		"一条网络信息，该不该信",       // the script
		"引导",                // the current phase
		phase.Goal,          // the phase goal
		phase.SoftCondition, // the condition being judged
		"这条是真的吧？",           // this phase's dialogue
		"CRAAP 卡：进行中",       // the active card instance
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("course context is missing %q:\n%s", want, got)
		}
	}
}

func TestProposeCourseReplyReturnsAReply(t *testing.T) {
	prov := scriptedProvider(`{"type":"reply","body":"你先说说，这条说法里哪一句最像是被加工过的？"}`)
	out, usage, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Type != "reply" || out.Body == "" {
		t.Fatalf("out = %+v, want a reply with a body", out)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated so the caller can meter the call")
	}
}

func TestProposeCourseReplyReturnsAnAdvance(t *testing.T) {
	prov := scriptedProvider(`{"type":"advance","to":"independent"}`)
	out, _, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Type != "advance" || out.To != "independent" {
		t.Fatalf("out = %+v, want advance to independent", out)
	}
}

func TestProposeCourseReplyMetersARejectedOutput(t *testing.T) {
	// A ghostwriting reply must be rejected by the enforcement stack — but the
	// tokens were already spent, so usage must still come back for the caller
	// to record an llm_call row (the metering-on-reject rule). The brief's
	// literal string ("你可以这样写：...") does not match the banned-phrasing
	// corpus (internal/agent/enforcement/banned_phrasing.go only bans
	// "你应该这样写" / "应该这样写："), so this uses the corpus's actual
	// "rewritten-sentence-zh" rule phrase instead.
	prov := scriptedProvider(`{"type":"reply","body":"你应该这样写：中国的绿化成就无可否认。"}`)
	_, usage, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err == nil {
		t.Fatal("a ghostwriting reply must be rejected")
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated even on reject")
	}
}
