package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// TestCourseAskPrompt asserts BuildCourseAskPrompt names the course, the
// current step, and the course goal, and carries every 铁律 guardrail phrase
// the free-Q&A coach must never violate: restraint on quiz answers (①), no
// ghostwriting/conclusions-for-the-student (③... project rule, same posture
// as chat/studio), and one-question-at-a-time.
func TestCourseAskPrompt(t *testing.T) {
	got := BuildCourseAskPrompt(
		"一条网络信息，该不该信",
		"溯源体检",
		"学会用 CRAAP 给一条说法做信源辨识",
		"这一步会带学生检查作者、发布时间与目的……",
	)

	for _, want := range []string{
		"一条网络信息，该不该信",     // course title
		"溯源体检",             // step title
		"学会用 CRAAP 给一条说法做信源辨识", // course goal
		"测验",                // quiz-answer restraint mentions the quiz
		"答案",                // ...and the answer
		"不替他下结论",            // never conclude for the student
		"不替他写作",             // never write for the student
		"一次只问一个",            // one question at a time
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("course ask prompt is missing %q:\n%s", want, got)
		}
	}
}

func TestProposeCourseAskReplyReturnsAReply(t *testing.T) {
	prov := scriptedProvider("你先说说，这条说法的作者是谁？")
	out, usage, err := ProposeCourseAskReply(context.Background(), prov, gateway.Resolved{},
		"一条网络信息，该不该信", "溯源体检", "学会用 CRAAP 做信源辨识", "步骤内容摘要", "这条是真的吗？")
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

func TestProposeCourseAskReplyMetersARejectedOutput(t *testing.T) {
	// A ghostwriting reply must be rejected by the enforcement stack — but the
	// tokens were already spent, so usage must still come back so the caller
	// can still record an llm_call row (metering-on-reject).
	prov := scriptedProvider("你应该这样写：中国的绿化成就无可否认。")
	_, usage, err := ProposeCourseAskReply(context.Background(), prov, gateway.Resolved{},
		"一条网络信息，该不该信", "溯源体检", "学会用 CRAAP 做信源辨识", "步骤内容摘要", "帮我写结论")
	if err == nil {
		t.Fatal("a ghostwriting reply must be rejected")
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated even on reject")
	}
}
