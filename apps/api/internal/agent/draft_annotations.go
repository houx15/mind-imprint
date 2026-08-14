package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// draft_annotations.go — slice 3b · the 批注 reviewer. A flagship reasoning model
// reads the student's proposal and returns layered (paper/paragraph/sentence),
// colored (good=green/suggest=blue/problem=red) teacher annotations. It NEVER
// rewrites the student's text (铁律①) — each note is a direction. 批注 is
// deliberately sparse: paragraph/sentence marks only where warranted; green only
// at the paper level. Rendered view-only in the left panel (never overlaid on the
// editable draft).

// DraftAnnotationOut is one parsed 批注 (the server mints ids on persist).
type DraftAnnotationOut struct {
	Level   string `json:"level"`   // paper | paragraph | sentence
	Nature  string `json:"nature"`  // good | suggest | problem
	Quote   string `json:"quote"`   // sentence text; "" for paper
	Locator string `json:"locator"` // e.g. 第2段; "" for paper
	Note    string `json:"note"`
}

// DraftAnnotationInput is the whole draft (+ an optional per-step focus). Doc
// selects the document being 批注ed ("proposal" | "essay") so the prompt names it
// correctly; empty defaults to the proposal (back-compat).
type DraftAnnotationInput struct {
	Title string
	Draft string
	Focus string // the current guided step (for 我写好了); "" for a whole-draft check
	Doc   string // "proposal" | "essay"; "" = proposal
}

// annotationDocNoun names the document in the reviewer prompt.
func annotationDocNoun(doc string) string {
	if doc == "essay" {
		return "论文正文"
	}
	return "研究提案"
}

func draftAnnotationSystemFor(docNoun string) string {
	return `你是一位像老师一样批改` + docNoun + `的 IB 导师。通读学生写的` + docNoun + `，像老师用红蓝绿笔在纸上批注一样，给出分层的批注。批注要克制——只标你真的有话要说的地方，不要每段每句都标。绝不替学生改写正文，只给方向（铁律①）。

只返回一个 JSON 对象：
{"annotations":[{"level":"paper|paragraph|sentence","nature":"good|suggest|problem","quote":"（句子级：原文照抄的那句话；其它为空）","locator":"（段落级/句子级：如「第2段」；paper 为空）","note":"你的批注（一句到几句，是建议方向，不是改写）"}]}

分层规则：
- paper（整体）：给一段总体评价——至少一条做得好的地方（nature=good），以及最需要加强的一两点（nature=suggest 或 problem）。
- paragraph（段落）：只在某段确有可改进处时给（nature=suggest 或 problem，别用 good），带 locator，note 要具体、可举例说明。不要每段都评。
- sentence（句子）：只针对个别值得指出的句子/短语（nature=suggest 或 problem），quote 照抄原句。
- good（绿）只用于 paper 级；段落与句子只用 suggest（蓝）或 problem（红）。
- 批注总数不超过 10 条。用中文。只回 JSON，不要任何解释或代码块外的文字。`
}

const maxDraftAnnotationAttempts = 2
const maxDraftAnnotations = 10

// ReviewDraftAnnotations runs the flagship reviewer and returns the parsed 批注.
// Best-effort by contract: the caller degrades to an empty list on any error.
// Usage is returned so the caller meters even a failed call.
func ReviewDraftAnnotations(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in DraftAnnotationInput) ([]DraftAnnotationOut, gateway.ChatUsage, error) {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	if s := strings.TrimSpace(in.Focus); s != "" {
		fmt.Fprintf(&b, "（重点看这一部分：%s）\n", s)
	}
	fmt.Fprintf(&b, "学生写的提案：\n%s\n", strings.TrimSpace(in.Draft))

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: draftAnnotationSystemFor(annotationDocNoun(in.Doc))},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		// 16000 = the whole-draft-review budget (see gateway/deepseek.go): this IS a
		// whole-draft review on the flagship reasoning model, whose reasoning_content
		// eats the completion budget before the JSON answer is emitted. The old 3500
		// cap pinned completion_tokens at the limit on nearly every call → "no JSON
		// object in reply" → the reviewer silently returned an empty set (铁律-safe
		// degrade), so 批注 almost never appeared. Must match the large-output default.
		MaxTokens: 16000,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxDraftAnnotationAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		items, perr := parseDraftAnnotations(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return items, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("draft annotations: no parseable review")
	}
	return nil, lastUsage, lastErr
}

func validAnnotationLevel(s string) bool {
	return s == "paper" || s == "paragraph" || s == "sentence"
}

func validAnnotationNature(s string) bool {
	return s == "good" || s == "suggest" || s == "problem"
}

// parseDraftAnnotations decodes the model's JSON, drops items that are invalid /
// empty-note / banned-phrasing, and clamps to maxDraftAnnotations. An
// unparseable reply, or one with zero surviving items, is an error (drives the
// retry / caller degrade).
func parseDraftAnnotations(text string) ([]DraftAnnotationOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return nil, fmt.Errorf("draft annotations: no JSON object in reply")
	}
	var parsed struct {
		Annotations []DraftAnnotationOut `json:"annotations"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("draft annotations: unmarshal: %w", err)
	}
	out := make([]DraftAnnotationOut, 0, len(parsed.Annotations))
	for _, a := range parsed.Annotations {
		note := strings.TrimSpace(a.Note)
		if note == "" || !validAnnotationLevel(a.Level) || !validAnnotationNature(a.Nature) {
			continue
		}
		if enforcement.BannedPhrasing(note) != nil {
			continue // a 批注 that reads like a rewrite is dropped (铁律①), not fatal
		}
		out = append(out, DraftAnnotationOut{
			Level: a.Level, Nature: a.Nature,
			Quote: strings.TrimSpace(a.Quote), Locator: strings.TrimSpace(a.Locator), Note: note,
		})
		if len(out) == maxDraftAnnotations {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("draft annotations: no valid annotations in reply")
	}
	return out, nil
}
