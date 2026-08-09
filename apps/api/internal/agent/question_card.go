package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// question_card.go — slice 3a · the 提问卡 (question card) adaptive sub-agent
// (all-statuses.md §2). At the very start of inquiry it activates the student's
// own experience, intuition and questions before AI extends them — turning a
// vague/empty 目标 into a focused, personal research question. It runs a
// multi-turn conversation on the FAST model, one question at a time (铁律③); it
// never writes the research question FOR the student (铁律①) — SuggestedObjective
// is only the student's own articulated wording, echoed back for confirmation
// once Done.

// QuestionCardOut is one turn of the sub-agent.
type QuestionCardOut struct {
	Narrate            string `json:"narrate"`
	SuggestedObjective string `json:"suggestedObjective"`
	Done               bool   `json:"done"`
}

// QuestionCardInput carries the task + the conversation so far. Objective is the
// student's current (possibly empty/vague) 目标.
type QuestionCardInput struct {
	Title     string
	Objective string
	History   []ChatTurn
}

const questionCardSystem = `你是「印记」提问卡里的引导子代理。学生刚拿到一个题目，还没有自己的想法就想直接让 AI 给方向。你的任务不是给答案，而是激活他自己的经验与疑问，最后帮他把一个【属于他自己的、更聚焦的研究问题】说出来。一次只问一个问题（铁律③），绝不替他写研究问题（铁律①）。

按大致这个顺序、顺着学生的回答自然推进（不要机械照搬）：
1. 如果学生已给的目标与题目无关、或太笼统，先解释为什么要收窄：好的研究目标不是把题目换个说法复述；要说清「我将按什么理解来回答这个题目」；若关键词有多种解释，要选一种工作定义并说明理由。
2. 拆解题目：「用你自己的话说说，你对这个题目的理解是？」如果学生的理解完全不相关，用初中生能懂的话把题目翻译、解释一遍。
3. 问他看到这个题目会联想到什么经验/知识（要具体：一个具体例子、一份报告、一位艺术家……）。
4. 问他对那个经验的理解。
5. 引导他基于这个例子提出一个更具体的研究问题。

只返回一个 JSON 对象：
{"narrate": "给学生看的一句话（一次只问一个）", "suggestedObjective": "仅当对话可以收尾、且学生已用自己的话说出研究问题时，把他的措辞回显在这里；否则为空字符串", "done": false}

要求：
- 未收尾时 done=false 且 suggestedObjective 为空。
- 收尾时 done=true，suggestedObjective = 学生自己说出的研究问题（你的整理，但不改变他的意思，绝不凭空发明）。
- 用中文；只回 JSON，不要代码块外的任何文字。`

const maxQuestionCardAttempts = 2

// QuestionCardTurn runs one turn of the sub-agent. Best-effort: any error → the
// caller degrades to a fallback narrate. Usage is returned for metering.
func QuestionCardTurn(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in QuestionCardInput) (QuestionCardOut, gateway.ChatUsage, error) {
	msgs := make([]gateway.ChatMessage, 0, len(in.History)+2)
	sys := questionCardSystem
	if s := strings.TrimSpace(in.Title); s != "" {
		sys += "\n\n当前题目：" + s
	}
	if s := strings.TrimSpace(in.Objective); s != "" {
		sys += "\n学生当前的目标（可能太泛）：" + s
	}
	msgs = append(msgs, gateway.ChatMessage{Role: gateway.RoleSystem, Content: sys})
	for _, h := range in.History {
		role := gateway.RoleUser
		if h.Role == "assistant" {
			role = gateway.RoleAssistant
		}
		msgs = append(msgs, gateway.ChatMessage{Role: role, Content: h.Content})
	}
	req := gateway.ChatRequest{Messages: msgs, MaxTokens: 1500}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxQuestionCardAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parseQuestionCard(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("question card: no parseable reply")
	}
	return QuestionCardOut{}, lastUsage, lastErr
}

func parseQuestionCard(text string) (QuestionCardOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return QuestionCardOut{}, fmt.Errorf("question card: no JSON object in reply")
	}
	var o QuestionCardOut
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		return QuestionCardOut{}, fmt.Errorf("question card: unmarshal: %w", err)
	}
	if strings.TrimSpace(o.Narrate) == "" {
		return QuestionCardOut{}, fmt.Errorf("question card: empty narrate")
	}
	return o, nil
}
