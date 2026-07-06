package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// Anchor is the agent's view of a card anchor (mirrors the TS Anchor contract).
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
	Generate(ctx context.Context, spec cards.Spec, materials []Material) ([]Anchor, error)
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

// computeOffsets returns the byte offsets of quote within the block's text (0,0
// when not found — quote stays authoritative for the UI).
func computeOffsets(text, quote string) (int, int) {
	i := strings.Index(text, quote)
	if i < 0 {
		return 0, 0
	}
	return i, i + len(quote)
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
	return out, nil
}

// fallbackAnchors builds one unanchored ai anchor per card dimension (step title).
func fallbackAnchors(spec cards.Spec, materials []Material) []Anchor {
	matID := ""
	if len(materials) > 0 {
		matID = materials[0].ID
	}
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

func (g *llmAnchorGenerator) Generate(ctx context.Context, spec cards.Spec, materials []Material) ([]Anchor, error) {
	resolved, err := g.resolver(ctx)
	if err != nil {
		return fallbackAnchors(spec, materials), nil
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
		return fallbackAnchors(spec, materials), nil
	}
	anchors, perr := parseAnchorGen(res.Text, spec, materials)
	if perr != nil || len(anchors) == 0 {
		return fallbackAnchors(spec, materials), nil
	}
	return anchors, nil
}

func buildAnchorPrompt(spec cards.Spec) string {
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
