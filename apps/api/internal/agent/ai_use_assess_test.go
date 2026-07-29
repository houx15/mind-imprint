package agent

import (
	"strings"
	"testing"
)

// TestAssessReportInput_IncludesAIUse — S5: the student's AI-use self-report +
// objective record must reach the assessor's user turn so the responsible-AI-use
// lens is grounded in the student's own words, not only inferred from behavior.
func TestAssessReportInput_IncludesAIUse(t *testing.T) {
	in := AssessmentInput{
		AIUse: AIUseForAssessment{
			UsedFor:    "用 AI 澄清检索词、核对来源功能、追问论证",
			NotUsedFor: "没有让 AI 代写正文，也没有预测分数",
			RecordLine: "12 轮对话 · AI 提议 2 张卡（打开 1、跳过 1）· 打开 5 个来源 · 无代写正文、无预测分数",
		},
	}
	out := assessReportUserInput(in)
	for _, want := range []string{"AI 使用自述", "澄清检索词", "没有让 AI 代写正文", "12 轮对话"} {
		if !strings.Contains(out, want) {
			t.Fatalf("assessor input missing %q:\n%s", want, out)
		}
	}
}

// An empty AI-use statement renders nothing (chat/course, or a project where the
// student never authored one).
func TestAssessReportInput_NoAIUseBlockWhenEmpty(t *testing.T) {
	out := assessReportUserInput(AssessmentInput{})
	if strings.Contains(out, "AI 使用自述") {
		t.Fatalf("empty AI-use must not render a block:\n%s", out)
	}
}
