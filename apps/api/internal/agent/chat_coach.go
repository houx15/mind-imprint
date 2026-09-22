package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// chatCoachPosturePrompt is the Chat-variant system prompt (agent-spec §5.4):
// coach alone, guiding-not-answering, ONE notch more permissive than the
// Studio (may explain and inform, scoped — product §2.2) but never producing
// the student's assessed deliverable, one question at a time, restraint ladder.
const chatCoachPosturePrompt = `你是「思维印记」的自由对话陪练。你帮助学生理解概念、分析材料并作出自己的判断。

- 引导，不代答：你可以解释、可以科普（这是学习空间），但绝不替学生写出他要被评估的成品，绝不替他下结论。
- 一次只问一个：回复简短，不啰嗦，顺着学生的话往深里带一步。
- 克制：当学生已经在思考时，别打断；他需要帮助时，提供解释或不同题目的示例；不代写他要提交的正文。
- 当学生贴进一个来源链接、并把它当成论据时，先顺着他的点回应一句，再（由系统）把「信源辨识」作为一个邀请递上——是邀请，不是打断。

解释已回答的问题后可以结束，不必再邀请另一个例子。用自然完整的句子回应，不猜测此前有过什么约定。拒绝代写时简短说明可以怎样帮助，不批评学生态度，具体说明可以提供的帮助。

只输出你要对学生说的那段话本身，不要任何前缀、标签或格式。`

// BuildChatContext assembles the chat-variant coach user message: the recent
// conversation, an optional one-line thread-graph summary, an optional
// summary of the thread's COMPLETED cards (N3b Seam B — chat manufactures no
// post-submit turn of its own, so a just-finished card rides the student's
// NEXT message instead), and an optional classifier flag naming a fresh card
// moment. Kept pure for testing. Output is byte-identical to before this
// argument existed when cardSummary is empty.
func BuildChatContext(history []ChatTurn, threadSummary, cardSummary, flag string) string {
	var b strings.Builder
	b.WriteString("对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", who, t.Content))
	}
	if strings.TrimSpace(threadSummary) != "" {
		b.WriteString("\n此对话中已有的材料：" + threadSummary + "\n")
	}
	if strings.TrimSpace(cardSummary) != "" {
		b.WriteString("\n此对话中已完成的工具卡：" + cardSummary + "\n")
	}
	if strings.TrimSpace(flag) != "" {
		b.WriteString("\n刚刚发生的思考时机：" + flag + "\n")
	}
	b.WriteString("\n现在，简明回应学生最新的发言；需要解释时可用两三句，最多一个问题，允许不提问。")
	return b.String()
}

// ProposeChatReply asks the coach model (mid-tier chaperone, downgradeable) for one conversational reply, then
// runs the chat enforcement subset: ValidateOutput (reply shape) + BannedPhrasing.
// There is no OutputCheck echo pass — chat has no draft to echo. Usage is
// populated whenever Collect succeeded, INCLUDING when enforcement then rejects
// (a rejected reply still cost money; the caller must still meter it).
func ProposeChatReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, history []ChatTurn, threadSummary, cardSummary, flag string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: chatCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildChatContext(history, threadSummary, cardSummary, flag)},
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
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: chat reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}
