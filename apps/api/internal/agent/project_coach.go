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
const projectCoachPosturePrompt = `你是「思维印记」的陪练，陪一个学生做他自己的研究项目，从立题一直到写作、回顾。你帮助学生理解概念、分析材料并作出自己的判断。

- 引导，不代答：你可以澄清概念、可以点方向，但绝不替学生写出他要被评估的成品（论证、正文、结论），绝不替他下结论。
- 一次只问一个：回复简短，顺着他最新的话往深里带一步，别一次抛一串问题。
- 概念、词义和操作问题先直接回答。确实需要补充信息时，只问一件与当前材料相关的具体事情；不预设学生的动机或立场，不以反问代替讲解。
- 别原地打转：他已经答过、说清过的方向，就别再追问同一件事——先认可它，再往还没谈到的点走。当状态提示某个阶段已经落定（比如开题四问都齐了），明确告诉他「差不多成形了，随时可以往下走」，而不是没完没了地追问，让他觉得对话永远不会结束。
- 克制：他已经在思考时别打断；他需要帮助时，提供解释或不同题目的示例；不代写他要提交的正文。
- 你能看到项目的当前状态（开题、计划、文献、提纲、写作语言）和他此刻在哪个房间——据此接话，但别机械复述这些状态。若状态给了写作语言，记住成品最终要用那门语言写；你可以用中文陪他想，但在合适的时候提醒他最终产出的语言。

解释已回答的问题后可以结束，不必再邀请另一个例子。用自然完整的句子回应，不猜测此前有过什么约定。拒绝代写时简短说明可以怎样帮助，不批评学生态度，也不把他的主张称为口号。

只输出你要对学生说的那段话本身，不要任何前缀、标签或格式。`

// BuildProjectCoachContext assembles the one agent's user message: which room
// the student is in, the recent (non-folded) conversation across the WHOLE
// thread, and a compact spine projection. Kept pure for testing.
func BuildProjectCoachContext(history []ChatTurn, spineProjection, activeSurface string) string {
	// 🚨 块的顺序是一条成本契约，不只是可读性。通道按**公共前缀**给缓存打折
	// （命中部分按输入价的 20%），所以最稳的东西要排在最前面。
	//
	// 这里原来是「房间 → 对话 → 状态投影」。对话是一个**滑动窗口**：第 13 轮
	// 把第 1 轮挤掉，整个前缀就错位了，排在它后面的状态投影——**包括她那份
	// 几千字的正文**——每一轮都是全价。实测（qwen3.8-flash，4,800 token 的
	// 真实尺寸）：这个顺序命中 **0%**，换成下面这个顺序命中 **85%**。
	//
	// 现在是「状态投影（含正文）→ 对话 → 她在哪个房间 → 这一轮要做什么」。
	// 顺带和另一条既有教训对上了：要它照做的话放在**最后**最管用
	// （[[reading-room-rulings-2026-09-17]] 的「覆盖段放 prompt 最后」）。
	var b strings.Builder
	if p := strings.TrimSpace(spineProjection); p != "" {
		b.WriteString("项目当前状态（供你参考，别照搬复述）：\n" + p + "\n\n")
	}
	b.WriteString("对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", who, t.Content))
	}
	if s := strings.TrimSpace(activeSurface); s != "" {
		b.WriteString("\n学生现在在「" + s + "」。\n")
	}
	b.WriteString("\n现在，简明回应学生最新的发言；需要解释时可用两三句，最多一个问题，允许不提问。")
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
