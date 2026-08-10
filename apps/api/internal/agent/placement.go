package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// placement.go — 印记 for one reading source suggests the best-fit research
// question to hang it under (or none → 未归类). Fast model, reasoning-off.
// Advisory (铁律②): the student confirms the placement with a tap. Best-effort:
// any error → caller degrades to "no suggestion" (未归类).

type PlacementQuestion struct {
	ID   string
	Text string
}

type SuggestPlacementInput struct {
	Title     string
	Abstract  string
	Journal   string
	Year      string
	Questions []PlacementQuestion
}

// PlacementSuggestionOut.LeadID == "" means "no good fit → 未归类".
type PlacementSuggestionOut struct {
	LeadID string `json:"leadId"`
	Reason string `json:"reason"`
}

const placementSystem = `你是一位 IB 研究导师。学生刚加进一篇文献。下面给你这篇文献的信息，和这个项目的若干【研究问题】（每个带一个 id）。请判断这篇文献最能支撑/回答哪一个问题。

只返回一个 JSON 对象：
{"leadId": "最贴合的问题 id，或 null", "reason": "一句话中文：为什么挂这个问题（或为什么都不太贴）"}

要求：
- leadId 必须是给定问题里的某个 id；如果都不太贴，返回 null。
- reason 一句话，具体，给学生看的。
- 只回 JSON，不要任何解释或代码块外的文字。`

const maxPlacementAttempts = 2

func SuggestBestQuestion(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in SuggestPlacementInput) (PlacementSuggestionOut, gateway.ChatUsage, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "文献标题：%s\n", strings.TrimSpace(in.Title))
	if s := strings.TrimSpace(in.Journal); s != "" {
		fmt.Fprintf(&b, "期刊/来源：%s\n", s)
	}
	if s := strings.TrimSpace(in.Year); s != "" {
		fmt.Fprintf(&b, "年份：%s\n", s)
	}
	if s := strings.TrimSpace(in.Abstract); s != "" {
		fmt.Fprintf(&b, "摘要：%s\n", s)
	}
	b.WriteString("\n研究问题：\n")
	for _, q := range in.Questions {
		fmt.Fprintf(&b, "  - id=%s  %s\n", q.ID, strings.TrimSpace(q.Text))
	}

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: placementSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 600,
	}

	valid := map[string]bool{}
	for _, q := range in.Questions {
		valid[q.ID] = true
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxPlacementAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parsePlacement(res.Text, valid)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("placement: no parseable suggestion")
	}
	return PlacementSuggestionOut{}, lastUsage, lastErr
}

func parsePlacement(text string, valid map[string]bool) (PlacementSuggestionOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return PlacementSuggestionOut{}, fmt.Errorf("placement: no JSON object in reply")
	}
	var parsed struct {
		LeadID *string `json:"leadId"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return PlacementSuggestionOut{}, fmt.Errorf("placement: unmarshal: %w", err)
	}
	out := PlacementSuggestionOut{Reason: strings.TrimSpace(parsed.Reason)}
	if parsed.LeadID != nil {
		id := strings.TrimSpace(*parsed.LeadID)
		if valid[id] { // drop hallucinated / null ids → "" (未归类)
			out.LeadID = id
		}
	}
	return out, nil
}
