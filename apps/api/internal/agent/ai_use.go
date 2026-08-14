package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// ai_use.go — S5 · the SEED half of the AI-interaction retrospective. From the
// objective interaction record (assembled server-side, never a model output),
// seed a FIRST-PERSON draft of the student's AI-use statement — used-for and
// not-used-for. It is only a SEED: the student rewrites it (AI 克制 — the
// reflection must be her own; the AI never writes it). Mirrors ComposeDigestMerge:
// pure input → one isolated mid-tier call → struct. The parse target carries
// ONLY the two authored fields, so the model can never echo or forge the
// objective record (which stays server-computed).

// AIUseRecordView is the objective record, projected for the seed prompt.
type AIUseRecordView struct {
	CoachTurns        int
	CardsProposed     int
	CardsAccepted     int
	CardsDismissed    int
	SourcesOpened     int
	LLMCallsByPurpose map[string]int
}

func (v AIUseRecordView) empty() bool {
	return v.CoachTurns == 0 && v.CardsProposed == 0 && v.SourcesOpened == 0 && len(v.LLMCallsByPurpose) == 0
}

const aiUseSeedSystem = `你在帮一个学生起草「我是怎么用 AI 的」自述——只是给一个初稿，学生会自己改写。依据下面这份客观交互记录，用第一人称写两段：
- used_for：AI 在这个项目里真正帮你做了什么（澄清检索词、核对来源功能、追问论证、检查过度概括、答辩追问等）。
- not_used_for：你明确没有让 AI 做什么（代写正文、编造材料细节、预测分数、替你写反思等）。
绝不夸大 AI 的作用，绝不把「思考」说成是 AI 做的——AI 是过程工具，不是代写者。每段一两句话。只输出 JSON：{"used_for":"...","not_used_for":"..."}。`

// ComposeAIUseSeed seeds the student's AI-use draft from the objective record via
// one isolated mid-tier call. An empty record → ("", "", zero usage, nil) with
// ZERO provider calls (no-spend gate). Usage is returned whenever Collect
// succeeded so the caller can meter a completed call.
func ComposeAIUseSeed(ctx context.Context, prov gateway.Provider, r gateway.Resolved, rec AIUseRecordView) (string, string, gateway.ChatUsage, error) {
	if rec.empty() {
		return "", "", gateway.ChatUsage{}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "对话轮数：%d\n", rec.CoachTurns)
	fmt.Fprintf(&b, "AI 提议的工具卡：%d（你打开 %d、跳过 %d）\n", rec.CardsProposed, rec.CardsAccepted, rec.CardsDismissed)
	fmt.Fprintf(&b, "你查阅的来源：%d\n", rec.SourcesOpened)
	if len(rec.LLMCallsByPurpose) > 0 {
		b.WriteString("AI 调用（按用途）：")
		for purpose, n := range rec.LLMCallsByPurpose {
			fmt.Fprintf(&b, "%s×%d ", purpose, n)
		}
		b.WriteString("\n")
	}
	b.WriteString("（记录里没有任何「AI 代写正文」或「AI 预测分数」的事件。）\n")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: aiUseSeedSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return "", "", gateway.ChatUsage{}, err
	}

	// 克制 at the type level: the parse target holds ONLY the two authored
	// fields — the model cannot carry the objective record back.
	var out struct {
		UsedFor    string `json:"used_for"`
		NotUsedFor string `json:"not_used_for"`
	}
	if err := json.Unmarshal([]byte(stripFences(res.Text)), &out); err != nil {
		return "", "", res.Usage, fmt.Errorf("agent: ai-use seed parse: %w", err)
	}
	return strings.TrimSpace(out.UsedFor), strings.TrimSpace(out.NotUsedFor), res.Usage, nil
}
