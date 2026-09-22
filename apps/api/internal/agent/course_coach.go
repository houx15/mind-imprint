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
