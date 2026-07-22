package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// Anchor is the agent's view of a card anchor (mirrors the TS Anchor contract).
// Start/End are rune (Unicode code point) indices into the block's text, not
// byte offsets — see computeOffsets.
type Anchor struct {
	ID         string `json:"id"`
	MaterialID string `json:"material_id"`
	BlockID    string `json:"block_id"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Quote      string `json:"quote"`
	Dimension  string `json:"dimension"`
	Author     string `json:"author"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
}

// AnchorGenerator produces material-anchored guiding questions for an annotation card.
type AnchorGenerator interface {
	Generate(ctx context.Context, spec cards.Spec, materials []Material) (GenerateResult, error)
}

// GenerateResult is Generate's return value: the anchors plus the
// provider/model/tier and token usage of the LLM call that produced them
// (design's "记录档位 + token + 成本" hard constraint — this is one of the
// three live gateway.Collect call sites, agent/coach.go and agent/course.go
// being the other two). Resolved.Provider == "" means no real call was made
// (the resolver errored, or Collect itself errored before any tokens were
// spent) — the deterministic fallbackAnchors ran instead, so there is
// nothing to record. A real call's Usage is populated even when the model's
// reply then fails to parse and Generate falls back to fallbackAnchors: the
// call still cost money, so callers must still record it.
type GenerateResult struct {
	Anchors  []Anchor
	Resolved gateway.Resolved
	Usage    gateway.ChatUsage
}

type llmAnchorGenerator struct {
	provider gateway.Provider
	resolver gateway.KeyResolver
}

// NewAnchorGenerator returns the production generator (LLM Collect + parse, with
// a deterministic per-dimension fallback).
func NewAnchorGenerator(provider gateway.Provider, resolver gateway.KeyResolver) AnchorGenerator {
	return &llmAnchorGenerator{provider: provider, resolver: resolver}
}

// genItem is the model's per-anchor JSON contract (offsets are computed server-side).
type genItem struct {
	BlockID   string `json:"block_id"`
	Quote     string `json:"quote"`
	Dimension string `json:"dimension"`
	Question  string `json:"question"`
}

func stripFences(text string) string {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	return c
}

// blockLookup indexes material blocks by block id → (materialID, text).
func blockLookup(materials []Material) map[string][2]string {
	m := map[string][2]string{}
	for _, mat := range materials {
		for _, b := range mat.Blocks {
			m[b.ID] = [2]string{mat.ID, b.Text}
		}
	}
	return m
}

// computeOffsets returns RUNE (Unicode code point) indices, not byte offsets:
// the web consumes these to slice the same block text, and one CJK character
// is 3 bytes but 1 rune. Returning byte offsets highlighted the wrong
// sentence on every Chinese material (spec §7.1). (0,0) when not found —
// quote stays authoritative for the UI.
func computeOffsets(text, quote string) (int, int) {
	i := strings.Index(text, quote)
	if i < 0 {
		return 0, 0
	}
	start := utf8.RuneCountInString(text[:i])
	return start, start + utf8.RuneCountInString(quote)
}

func parseAnchorGen(text string, spec cards.Spec, materials []Material) ([]Anchor, error) {
	var items []genItem
	if err := json.Unmarshal([]byte(stripFences(text)), &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errString("empty anchors")
	}
	lut := blockLookup(materials)
	out := make([]Anchor, 0, len(items))
	for i, it := range items {
		ref, ok := lut[it.BlockID]
		if !ok {
			return nil, errString("unknown block_id: " + it.BlockID)
		}
		start, end := computeOffsets(ref[1], it.Quote)
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: ref[0], BlockID: it.BlockID,
			Start: start, End: end, Quote: it.Quote, Dimension: it.Dimension,
			Author: "ai", Question: it.Question, Answer: "",
		})
	}
	// C2 annotate cards (params.tags present) require the model's dimension
	// vocabulary to exactly cover the completion tags, else
	// EvaluateCompletion's every_tag_present can never be satisfied and the
	// student's filled-in card silently never mints. A well-formed reply with
	// an off-vocabulary dimension (or a missing tag) is treated as a parse
	// failure here so Generate falls back to the tag-correct
	// fallbackAnchors. Legacy cards (no params.tags) are unvalidated.
	if len(spec.Params.Tags) > 0 {
		wantTags := map[string]bool{}
		for _, tag := range spec.Params.Tags {
			wantTags[tag] = false
		}
		for _, a := range out {
			if _, ok := wantTags[a.Dimension]; !ok {
				return nil, errString("anchor dimension not in tag vocabulary: " + a.Dimension)
			}
			wantTags[a.Dimension] = true
		}
		for tag, seen := range wantTags {
			if !seen {
				return nil, errString("missing anchor for tag: " + tag)
			}
		}
	}
	return out, nil
}

// fallbackAnchors builds one unanchored ai anchor per card dimension. C2
// annotate cards (params.tags present) key anchors off the completion tags so
// EvaluateCompletion's every_tag_present is satisfiable; legacy cards (no C2
// params block) key off step titles as before.
func fallbackAnchors(spec cards.Spec, materials []Material) []Anchor {
	matID := ""
	if len(materials) > 0 {
		matID = materials[0].ID
	}
	// C2 annotate cards (params.tags present) key anchors off the completion
	// tags so EvaluateCompletion's every_tag_present is satisfiable. Guidance
	// level L1: the AI authors the question here; higher levels populate
	// anchors from student action upstream without touching this path.
	if len(spec.Params.Tags) > 0 {
		out := make([]Anchor, 0, len(spec.Params.Tags))
		for i, tag := range spec.Params.Tags {
			q := spec.Params.TagPrompts[tag]
			if q == "" {
				q = "从「" + tag + "」这个角度看这份材料，你注意到什么？"
			}
			out = append(out, Anchor{
				ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
				Dimension: tag, Author: "ai", Question: q, Answer: "",
			})
		}
		return out
	}
	// Legacy step-title path (cards without a C2 params block).
	out := make([]Anchor, 0, len(spec.Steps))
	for i, st := range spec.Steps {
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
			Dimension: st.Title, Author: "ai",
			Question: "从「" + st.Title + "」这个角度看这份材料，你注意到什么？",
			Answer:   "",
		})
	}
	return out
}

func (g *llmAnchorGenerator) Generate(ctx context.Context, spec cards.Spec, materials []Material) (GenerateResult, error) {
	resolved, err := g.resolver(ctx)
	if err != nil {
		return GenerateResult{Anchors: fallbackAnchors(spec, materials)}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildAnchorPrompt(spec)},
			{Role: gateway.RoleUser, Content: BuildMaterialContext(materials)},
		},
		MaxTokens: 1500,
	}
	res, err := gateway.Collect(ctx, g.provider, resolved, req)
	if err != nil {
		return GenerateResult{Anchors: fallbackAnchors(spec, materials)}, nil
	}
	// A real call succeeded — it cost money regardless of what parsing does
	// next, so Resolved/Usage are always attached from here on.
	out := GenerateResult{Resolved: resolved, Usage: res.Usage}
	anchors, perr := parseAnchorGen(res.Text, spec, materials)
	if perr != nil || len(anchors) == 0 {
		out.Anchors = fallbackAnchors(spec, materials)
		return out, nil
	}
	out.Anchors = anchors
	return out, nil
}

func buildAnchorPrompt(spec cards.Spec) string {
	// C2 annotate cards (params.tags present) must key the model's dimension
	// output off the completion tags, else EvaluateCompletion's
	// every_tag_present can never be satisfied. Legacy cards (no C2 params
	// block) keep the step-title dimension vocabulary.
	if len(spec.Params.Tags) > 0 {
		var dims strings.Builder
		for _, tag := range spec.Params.Tags {
			dims.WriteString("- " + tag)
			if p := spec.Params.TagPrompts[tag]; p != "" {
				dims.WriteString("：" + p)
			}
			dims.WriteString("\n")
		}
		return "你是一名批判性思维教练。学生正在读下面这份材料。请针对「" + spec.Name +
			"」的每个维度，在材料里挑出一处最相关的原句，提出一个指向那句话的具体引导问题。\n" +
			"维度：\n" + dims.String() +
			"\n只输出 JSON 数组，每个元素形如 {\"block_id\":\"b0\",\"quote\":\"材料里的原句片段\",\"dimension\":\"维度名\",\"question\":\"你的问题\"}。" +
			"dimension 字段必须恰好是以下之一：" + strings.Join(spec.Params.Tags, "、") + "。" +
			"block_id 必须来自材料，quote 必须是该 block 里的原文片段。不要输出任何多余文字。"
	}
	var dims strings.Builder
	for _, st := range spec.Steps {
		dims.WriteString("- " + st.Title + "\n")
	}
	return "你是一名批判性思维教练。学生正在读下面这份材料。请针对「" + spec.Name +
		"」的每个维度，在材料里挑出一处最相关的原句，提出一个指向那句话的具体引导问题。\n" +
		"维度：\n" + dims.String() +
		"\n只输出 JSON 数组，每个元素形如 {\"block_id\":\"b0\",\"quote\":\"材料里的原句片段\",\"dimension\":\"维度名\",\"question\":\"你的问题\"}。" +
		"block_id 必须来自材料，quote 必须是该 block 里的原文片段。不要输出任何多余文字。"
}

// errString is a tiny error helper (avoids importing errors just for this file).
type errString string

func (e errString) Error() string { return string(e) }
