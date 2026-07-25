package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// course_coach.go — the Course-variant coach (agent-spec §5.3). Course is the
// most permissive of the three postures: teaching IS the point here, so the
// coach may explain and demonstrate — but it still never produces the
// student's assessed deliverable, and the consolidation rule holds. Runs on
// the chaperone (mid-tier) resolver like the chat coach, not the flagship.
const courseCoachPosturePrompt = `你是「思维印记」的课程陪练。你在带一节课，按脚本走。

- 可以讲解、可以演示：这是学习空间，把方法讲清楚是你的职责。但绝不替学生写出他要被评估的成品，绝不替他下结论。
- 一次只问一个：回复简短，顺着学生的话往深里带一步。
- 克制：当学生已经在思考时，别打断；当他想让你替他想时，把问题还给他。
- 工具卡的「框架」在用过之后才揭示，不在用之前——先做，再命名。
- 你不能重排或跳过阶段，不能替换这一阶段声明的卡。

输出必须是一个 JSON 对象，只能是下面两种之一：
{"type":"reply","body":"你要对学生说的话"}
{"type":"advance","to":"下一阶段的 id"}

只有当这一阶段的完成条件真的达成时，才输出 advance；否则输出 reply，把还差的那一步说给学生。`

// CourseScript is the session script the coach is executing: what course this
// is, its phases in binding order, and where we are.
type CourseScript struct {
	CourseTitle string
	PhaseTitles []string
	CurrentIdx  int
}

// BuildCourseContext is the course context recipe (agent-spec §5.3) and
// nothing more: the session script + the current phase goal + this phase's
// dialogue + the active card instance. Explicitly NOT the project graph — a
// course's graph is small and session-scoped. Pure; its own test.
func BuildCourseContext(script CourseScript, phase skills.Contract, history []ChatTurn, cardSummary, intentHint string) string {
	var b strings.Builder
	b.WriteString("课程：" + script.CourseTitle + "\n")
	b.WriteString("脚本（阶段顺序，不可重排）：" + strings.Join(script.PhaseTitles, " → ") + "\n")
	if script.CurrentIdx >= 0 && script.CurrentIdx < len(script.PhaseTitles) {
		b.WriteString("当前阶段：" + script.PhaseTitles[script.CurrentIdx] + "\n")
	}
	b.WriteString("这一阶段的目标：" + phase.Goal + "\n")
	if phase.SoftCondition != "" {
		b.WriteString("这一阶段的完成条件：" + phase.SoftCondition + "\n")
	}
	if cardSummary != "" {
		b.WriteString("当前工具卡：" + cardSummary + "\n")
	}
	b.WriteString("\n这一阶段的对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(who + "：" + t.Content + "\n")
	}
	switch intentHint {
	case "advance":
		b.WriteString("\n学生点了「继续」，这一阶段的完成条件已经满足，正在进入下一阶段。用 reply 写一句简短、温暖的过渡话：先肯定他这一阶段的思考，再自然地带到下一步。\n")
	default:
		b.WriteString("\n学生问了你一个问题。用 reply 回应他。\n")
	}
	return b.String()
}

// ProposeCourseReply runs one course-coach call through the full enforcement
// stack. Mirrors ProposeChatReply: no anchor, no criterion, no OutputCheck
// echo pass (Course has no draft to echo). Usage is returned even on reject —
// the tokens were spent, so the caller must still meter the call.
func ProposeCourseReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, ctxStr string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: courseCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: ctxStr},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, gateway.ChatUsage{}, err
	}
	// From here on, usage is non-zero and MUST travel with every return.
	usage := res.Usage
	var out enforcement.AgentOutput
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("course coach: parse output: %w", err)
	}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: course reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}
