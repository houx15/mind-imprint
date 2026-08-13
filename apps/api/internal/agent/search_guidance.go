package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// search_guidance.go — slice 5 (§113/§116) · 印记 proposes 2–3 search directions
// in the reading room, each a keyword + a one-line "why", from the research
// question + sub-questions + the student's 还需要探索的 notes. Fast model. Advisory
// (铁律②): the student runs a suggestion with one click, or ignores it. The reading
// room's 检索方向审视 posture — help pick WHERE to look, never fetch for the student.

type SearchSuggestionOut struct {
	Keyword string `json:"keyword"`
	Why     string `json:"why"`
}

type SearchGuidanceInput struct {
	Title        string
	Question     string
	SubQuestions []SubQuestion
	Needs        []string // the 还需要探索的 notes
	// FocusQuestion is the specific line the student is exploring right now — the
	// text of the question layer they're currently inside (a root question's hole,
	// or a selected sub-question). When set, the directions bias toward THIS layer
	// instead of covering the whole topic (item 3.1: directions tied to where the
	// student is).
	FocusQuestion string
}

const searchGuidanceSystem = `你是一位擅长文献检索的 IB 研究导师。学生在探索文献，请给出 2–3 个【检索方向】——每个是一个可以直接搜的关键词/词组，加一句话说明为什么这个方向值得搜（它能帮学生解决哪个子问题/补哪块证据）。

只返回一个 JSON 对象：
{"suggestions": [{"keyword": "可直接检索的英文或中文关键词", "why": "一句话：为什么搜这个方向"}, ...]}

要求：
- 2–3 条，聚焦、互不重复，贴着学生的研究问题和子问题。
- keyword 是能直接放进检索框的词，不是一句话。
- why 具体，指向某个子问题或某块缺的证据（尤其优先覆盖学生"还需要探索的"清单）。
- 若给出了"学生此刻正在探索这一层"，请把方向优先贴着这一层（这个具体的问题分支）来给，而不是泛泛覆盖整个题目。
- 只回 JSON，不要任何解释或代码块外的文字。`

const maxSearchGuidanceAttempts = 2

// ProposeSearchKeywords runs the fast generator. Best-effort: any error → the
// caller degrades to an empty list. Usage returned so the caller meters.
func ProposeSearchKeywords(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in SearchGuidanceInput) ([]SearchSuggestionOut, gateway.ChatUsage, error) {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	if s := strings.TrimSpace(in.Question); s != "" {
		fmt.Fprintf(&b, "研究问题：%s\n", s)
	}
	if len(in.SubQuestions) > 0 {
		b.WriteString("子问题：\n")
		for i, sq := range in.SubQuestions {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(sq.Text))
		}
	}
	if s := strings.TrimSpace(in.FocusQuestion); s != "" {
		fmt.Fprintf(&b, "学生此刻正在探索这一层：%s\n", s)
	}
	if len(in.Needs) > 0 {
		b.WriteString("学生还需要探索的：\n")
		for i, n := range in.Needs {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(n))
		}
	}

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: searchGuidanceSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 1500,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxSearchGuidanceAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parseSearchGuidance(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("search guidance: no parseable suggestions")
	}
	return nil, lastUsage, lastErr
}

func parseSearchGuidance(text string) ([]SearchSuggestionOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return nil, fmt.Errorf("search guidance: no JSON object in reply")
	}
	var parsed struct {
		Suggestions []SearchSuggestionOut `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("search guidance: unmarshal: %w", err)
	}
	out := make([]SearchSuggestionOut, 0, len(parsed.Suggestions))
	for _, s := range parsed.Suggestions {
		kw := strings.TrimSpace(s.Keyword)
		if kw == "" {
			continue
		}
		out = append(out, SearchSuggestionOut{Keyword: kw, Why: strings.TrimSpace(s.Why)})
		if len(out) == 3 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("search guidance: empty suggestions")
	}
	return out, nil
}
