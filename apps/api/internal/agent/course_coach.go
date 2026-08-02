package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// course_coach.go — the Course-variant coach, rewritten for course v2's
// free-Q&A AI bar (Task 6). Course v2 has no phase runtime left (migration
// 0050 dropped course_session/course_step; Task 5 deleted course_step.go's
// old CourseStore/RunCourseStep along with it) — the coach no longer walks a
// script or emits {"type":"reply"|"advance"} JSON. It answers ONE free
// question about the CURRENT step and stops. Same restraint the old
// courseCoachPosturePrompt held (Course may explain/demonstrate — teaching
// IS the point — but never produces the student's assessed deliverable, and
// the enforcement subset (ValidateOutput + BannedPhrasing) still runs on the
// output), applied to a single-shot Q&A instead of a scripted turn.

// BuildCourseAskPrompt returns the free-Q&A coach's system prompt: it names
// the course, the current step, and the course's stated goal, states the
// posture (student asks freely, coach helps him think it through — not
// answers for him), and then the four 铁律 guardrails verbatim: ① never hand
// over this step's quiz answer, only help him judge it himself; ② never
// conclude or write for him; ③ one question at a time; ④ keep it short.
// stepText (the step's authored content, when available) rides along as
// grounding context ONLY — the prompt explicitly tells the model not to
// recite it back, just to use it to answer in-context.
func BuildCourseAskPrompt(courseTitle, stepTitle, courseGoal, stepText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "你是「印记」，正在陪一名学生上《%s》这门课的「%s」这一步。本课目标：%s。学生会自由提问，请简明地帮他把这一步想清楚。\n",
		courseTitle, stepTitle, courseGoal)
	b.WriteString("硬规则：① 不要直接给出本步测验题的正确答案，只引导他自己判断；② 不替他下结论、不替他写作；③ 一次只问一个问题；④ 回答简短。\n")
	if strings.TrimSpace(stepText) != "" {
		b.WriteString("\n这一步的内容（仅供你理解语境，不要逐字复述给学生）：\n" + stepText + "\n")
	}
	return b.String()
}

// ProposeCourseAskReply runs one free-Q&A course-coach turn: builds the
// prompt (BuildCourseAskPrompt), sends the student's question as the sole
// user turn (no history — a course-ask is a one-shot bar, not a thread), and
// runs the same enforcement subset ProposeChatReply does: ValidateOutput
// (reply shape) + BannedPhrasing. No JSON envelope from the model — the raw
// text IS the reply body, mirroring ProposeChatReply exactly. Usage is
// populated whenever Collect succeeded, INCLUDING when enforcement then
// rejects the reply — the tokens were already spent, so the caller must
// still meter the call.
func ProposeCourseAskReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, courseTitle, stepTitle, courseGoal, stepText, studentInput string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: BuildCourseAskPrompt(courseTitle, stepTitle, courseGoal, stepText)},
			{Role: gateway.RoleUser, Content: studentInput},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	out := enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(res.Text)}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: course ask reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}
