package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// claim_revision.go — slice 4b-2 · the flagship classifier for a sub-question
// edit at the 大纲 step (§133–134). A "rephrase" keeps the same underlying
// research target, so the claim's already-gathered materials and any writing
// still apply; a "total_change" asks a different question, so they no longer do
// and the student should know before it lands. Advisory only (铁律②) — the
// result is a warning, never a block, and the student's writing is never deleted.

// ClaimRevisionVerdictOut is the classifier's read.
type ClaimRevisionVerdictOut struct {
	Kind string `json:"kind"` // rephrase | total_change
	Why  string `json:"why"`
}

// ClaimRevisionInput is one sub-question's old vs new text (+ siblings for
// context so the model can judge whether the new one duplicates a neighbour).
type ClaimRevisionInput struct {
	Title    string
	OldText  string
	NewText  string
	Siblings []SubQuestion
}

const claimRevisionSystem = `你是一位严谨的 IB 研究导师。学生在写作阶段修改了一个子问题（论点）。请判断这是"措辞微调"还是"换成了一个全新的问题"。

判断标准：
- rephrase（措辞调整）：研究对象、要回答的问题本质不变，只是说法更清楚/更准确。原来搜集的材料、已经写的论述仍然适用。
- total_change（全新问题）：问的是另一件事、另一个对象、另一个角度。原来的材料和已写论述基本不再适用。

只返回一个 JSON 对象：
{"kind": "rephrase", "why": "一句话说明为什么"}

要求：
- kind 只能是 "rephrase" 或 "total_change"。
- why 用中文，一句话，直接对学生说明差别，不评价修改意愿或能力（尤其 total_change 要点出材料/论述为何不再适用）。
- 只回 JSON，不要任何解释或代码块外的文字。`

const maxClaimRevisionAttempts = 2

// ClassifyClaimRevision runs the flagship classifier over the edit. Best-effort
// by contract: on ANY error the caller degrades to "rephrase" (the safe,
// non-alarming default) — a model hiccup must never block the student's own
// edit. Usage is returned so the caller meters even a failed call.
func ClassifyClaimRevision(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ClaimRevisionInput) (ClaimRevisionVerdictOut, gateway.ChatUsage, error) {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	fmt.Fprintf(&b, "原来的子问题：%s\n新子问题：%s\n", strings.TrimSpace(in.OldText), strings.TrimSpace(in.NewText))
	if len(in.Siblings) > 0 {
		b.WriteString("其它子问题（用于判断是否与它们重复/冲突）：\n")
		for i, sq := range in.Siblings {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(sq.Text))
		}
	}

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: claimRevisionSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 2000,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxClaimRevisionAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		v, perr := parseClaimRevisionVerdict(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return v, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("claim revision: no parseable verdict")
	}
	return ClaimRevisionVerdictOut{}, lastUsage, lastErr
}

func parseClaimRevisionVerdict(text string) (ClaimRevisionVerdictOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return ClaimRevisionVerdictOut{}, fmt.Errorf("claim revision: no JSON object in reply")
	}
	var v ClaimRevisionVerdictOut
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return ClaimRevisionVerdictOut{}, fmt.Errorf("claim revision: unmarshal: %w", err)
	}
	v.Kind = strings.TrimSpace(v.Kind)
	if v.Kind != "rephrase" && v.Kind != "total_change" {
		return ClaimRevisionVerdictOut{}, fmt.Errorf("claim revision: bad kind %q", v.Kind)
	}
	if strings.TrimSpace(v.Why) == "" {
		return ClaimRevisionVerdictOut{}, fmt.Errorf("claim revision: empty why")
	}
	return v, nil
}
