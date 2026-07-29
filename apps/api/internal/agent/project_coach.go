package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// project_coach.go — S1's ONE continuous per-project coach. There is one agent,
// one session; the four rooms (计划/阅读/写作/回顾 …) are views, not separate
// agents. This is the always-reply conversational coach (mirrors chat_coach's
// ProposeChatReply enforcement) lifted to project scope: it sees the WHOLE
// thread (continuity) plus a compact spine projection, and is told which room
// the student is in. Card offers / interventions are deliberately OFF here —
// cross-phase card proposing is S4; the intervention loop (RunAgentStep) is
// left intact as its substrate.

// projectCoachPosturePrompt is the one agent's posture across every room. The
// active surface + spine state ride in the user message (BuildProjectCoachContext),
// so ONE posture adapts to each room instead of four scope-specific prompts.
// Restrained per the 四条铁律: guide don't answer, one question at a time, never
// write the assessed deliverable, never conclude for the student.
const projectCoachPosturePrompt = `你是「思维印记」的陪练，陪一个学生做他自己的研究项目，从立题一直到写作、回顾。你的职责不是给答案，而是在对的时刻把「思考」塞回给学生。

- 引导，不代答：你可以澄清概念、可以点方向，但绝不替学生写出他要被评估的成品（论证、正文、结论），绝不替他下结论。
- 一次只问一个：回复简短，顺着他最新的话往深里带一步，别一次抛一串问题。
- 克制：他已经在思考时别打断；他想让你替他想时，把问题还给他。
- 你能看到项目的当前状态（开题、计划、文献、提纲）和他此刻在哪个房间——据此接话，但别机械复述这些状态。

只输出你要对学生说的那段话本身，不要任何前缀、标签或格式。`

// BuildProjectCoachContext assembles the one agent's user message: which room
// the student is in, the recent (non-folded) conversation across the WHOLE
// thread, and a compact spine projection. Kept pure for testing.
func BuildProjectCoachContext(history []ChatTurn, spineProjection, activeSurface string) string {
	var b strings.Builder
	if s := strings.TrimSpace(activeSurface); s != "" {
		b.WriteString("学生现在在「" + s + "」。\n\n")
	}
	b.WriteString("对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", who, t.Content))
	}
	if p := strings.TrimSpace(spineProjection); p != "" {
		b.WriteString("\n项目当前状态（供你参考，别照搬复述）：\n" + p + "\n")
	}
	b.WriteString("\n现在，用一句话回应学生最新的发言。")
	return b.String()
}

// ProposeProjectCoachReply asks the coach model (mid-tier, downgradeable) for
// one conversational reply, then runs the chat enforcement subset
// (ValidateOutput + BannedPhrasing) — identical to ProposeChatReply, minus the
// card/flag machinery. Usage is populated whenever Collect succeeded, INCLUDING
// when enforcement then rejects (a rejected reply still cost money; the caller
// must still meter it).
func ProposeProjectCoachReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, history []ChatTurn, spineProjection, activeSurface string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: projectCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildProjectCoachContext(history, spineProjection, activeSurface)},
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
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: project coach reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}
