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
	Generate(ctx context.Context, spec cards.Spec, materials []Material, level GuidanceLevel) (GenerateResult, error)
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

// blockLookup indexes material blocks by MATERIAL-QUALIFIED block id
// ("matID:blockID") → (materialID, text). Block ids are only "stable within a
// material" (materialize.Segment) — a flat map keyed by the bare block id
// would collide across materials in a ≥2-material project, with the last
// material silently winning. The qualified key mirrors the label
// BuildMaterialContext renders, so a model reply that follows the L1
// instruction (which shows the qualified example "m0:b0") round-trips
// directly.
//
// Qualified-only, deliberately: BuildMaterialContext (the L1 user message)
// always renders qualified labels and the L1 instruction always demonstrates
// the qualified form, so the model is never shown a bare id to imitate. A
// bare block_id in a reply has no legitimate producer — resolving it via a
// "there's only one material so guess" fallback would silently paper over a
// malformed reply instead of surfacing it, which is exactly the failure mode
// this function exists to close. An unqualified id fails closed via the
// caller's "unknown block_id" error.
func blockLookup(materials []Material) map[string][2]string {
	m := map[string][2]string{}
	for _, mat := range materials {
		for _, b := range mat.Blocks {
			m[mat.ID+":"+b.ID] = [2]string{mat.ID, b.Text}
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

func parseAnchorGen(text string, spec cards.Spec, materials []Material, level GuidanceLevel) ([]Anchor, error) {
	var items []genItem
	if err := json.Unmarshal([]byte(stripFences(text)), &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errString("empty anchors")
	}
	out := make([]Anchor, 0, len(items))
	if level == GuidanceL1 {
		lut := blockLookup(materials)
		for i, it := range items {
			ref, ok := lut[it.BlockID]
			if !ok {
				return nil, errString("unknown block_id: " + it.BlockID)
			}
			// The model returns the material-qualified id ("matID:blockID",
			// matching the [m.ID:blk.ID] label BuildMaterialContext renders).
			// Split on the LAST ':' to recover the material-local block id —
			// that is the shape the web already consumes to slice block
			// text, so only the lookup key changes, not what gets stored.
			localBlockID := it.BlockID
			if idx := strings.LastIndex(it.BlockID, ":"); idx >= 0 {
				localBlockID = it.BlockID[idx+1:]
			}
			start, end := computeOffsets(ref[1], it.Quote)
			out = append(out, Anchor{
				ID: "a" + strconv.Itoa(i), MaterialID: ref[0], BlockID: localBlockID,
				Start: start, End: end, Quote: it.Quote, Dimension: it.Dimension,
				Author: "ai", Question: it.Question, Answer: "",
			})
		}
	} else {
		// L2: the model supplies {dimension, question} only (buildAnchorPrompt
		// never asks it for block_id/quote at this level) — the span is hers to
		// locate, not the AI's to author (spec §4), so blockLookup/computeOffsets
		// are skipped entirely and the span stays blank. L3 never reaches here:
		// Generate returns before any model call at GuidanceL3.
		matID := ""
		if len(materials) > 0 {
			matID = materials[0].ID
		}
		for i, it := range items {
			out = append(out, Anchor{
				ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
				Start: 0, End: 0, Quote: "", Dimension: it.Dimension,
				Author: "student", Question: it.Question, Answer: "",
			})
		}
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

// fallbackAnchors builds one unanchored anchor per card dimension. C2
// annotate cards (params.tags present) key anchors off the completion tags so
// EvaluateCompletion's every_tag_present is satisfiable; legacy cards (no C2
// params block) key off step titles as before. It is level-aware so a
// degraded surface degrades to the RIGHT level rather than silently back to
// L1 (spec §4): L1 → author "ai", question present; L2 → author "student",
// same tag-prompt question text (span is blank at every level in this
// fallback path already); L3 → author "student", question blank — dimension
// only, nothing for her to be handed.
func fallbackAnchors(spec cards.Spec, materials []Material, level GuidanceLevel) []Anchor {
	matID := ""
	if len(materials) > 0 {
		matID = materials[0].ID
	}
	author := "ai"
	if level != GuidanceL1 {
		author = "student"
	}
	tagQuestion := func(tag string) string {
		if level == GuidanceL3 {
			return ""
		}
		q := spec.Params.TagPrompts[tag]
		if q == "" {
			q = "从「" + tag + "」这个角度看这份材料，你注意到什么？"
		}
		return q
	}
	if len(spec.Params.Tags) > 0 {
		out := make([]Anchor, 0, len(spec.Params.Tags))
		for i, tag := range spec.Params.Tags {
			out = append(out, Anchor{
				ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
				Dimension: tag, Author: author, Question: tagQuestion(tag), Answer: "",
			})
		}
		return out
	}
	// Legacy step-title path (cards without a C2 params block).
	out := make([]Anchor, 0, len(spec.Steps))
	for i, st := range spec.Steps {
		q := ""
		if level != GuidanceL3 {
			q = "从「" + st.Title + "」这个角度看这份材料，你注意到什么？"
		}
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
			Dimension: st.Title, Author: author,
			Question: q,
			Answer:   "",
		})
	}
	return out
}

func (g *llmAnchorGenerator) Generate(ctx context.Context, spec cards.Spec, materials []Material, level GuidanceLevel) (GenerateResult, error) {
	// L3: she elicits AND locates — there is no scaffold left for the AI to
	// generate, so no model call is made at all. This returns before
	// resolving a key or calling the provider. This is deliberate and is NOT
	// an instance of the "bailed out before metering" defect class this repo
	// has hit before: that class is about a call that HAPPENED and went
	// unrecorded (e.g. surfaceAnchors/renderChallenge returning early after a
	// real Collect). Here no call happens at all, so there is nothing to
	// meter — Resolved stays the zero value (Provider == "") on purpose
	// (spec §4).
	if level == GuidanceL3 {
		return GenerateResult{Anchors: fallbackAnchors(spec, materials, level)}, nil
	}
	resolved, err := g.resolver(ctx)
	if err != nil {
		return GenerateResult{Anchors: fallbackAnchors(spec, materials, level)}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildAnchorPrompt(spec, level)},
			{Role: gateway.RoleUser, Content: BuildMaterialContext(materials)},
		},
		MaxTokens: 1500,
	}
	res, err := gateway.Collect(ctx, g.provider, resolved, req)
	if err != nil {
		return GenerateResult{Anchors: fallbackAnchors(spec, materials, level)}, nil
	}
	// A real call succeeded — it cost money regardless of what parsing does
	// next, so Resolved/Usage are always attached from here on.
	out := GenerateResult{Resolved: resolved, Usage: res.Usage}
	anchors, perr := parseAnchorGen(res.Text, spec, materials, level)
	if perr != nil || len(anchors) == 0 {
		out.Anchors = fallbackAnchors(spec, materials, level)
		return out, nil
	}
	out.Anchors = anchors
	return out, nil
}

func buildAnchorPrompt(spec cards.Spec, level GuidanceLevel) string {
	// C2 annotate cards (params.tags present) must key the model's dimension
	// output off the completion tags, else EvaluateCompletion's
	// every_tag_present can never be satisfied. Legacy cards (no C2 params
	// block) keep the step-title dimension vocabulary. GuidanceL3 never
	// builds a prompt — Generate returns before calling the model.
	var dims strings.Builder
	tagged := len(spec.Params.Tags) > 0
	if tagged {
		for _, tag := range spec.Params.Tags {
			dims.WriteString("- " + tag)
			if p := spec.Params.TagPrompts[tag]; p != "" {
				dims.WriteString("：" + p)
			}
			dims.WriteString("\n")
		}
	} else {
		for _, st := range spec.Steps {
			dims.WriteString("- " + st.Title + "\n")
		}
	}

	if level == GuidanceL2 {
		// L2: she will locate the sentence herself, so the question must
		// stand on its own — it must NOT quote or point at a specific
		// sentence, or it would hand her the answer to the locating task
		// (spec §4). Only {dimension, question} is asked for; no block_id/
		// quote, so parseAnchorGen skips block/offset resolution entirely.
		instr := "你是一名批判性思维教练。学生正在读下面这份材料。请针对「" + spec.Name +
			"」的每个维度，提出一个具体的引导问题，帮助她自己去材料里找到相关的句子并思考。\n" +
			"维度：\n" + dims.String() +
			"\n重要：这次不要引用材料原句，也不要指出具体是哪一句——学生要自己在材料里找到答案对应的句子。问题必须能独立成立，不依赖你替她定位。\n" +
			"只输出 JSON 数组，每个元素形如 {\"dimension\":\"维度名\",\"question\":\"你的问题\"}。"
		if tagged {
			instr += "dimension 字段必须恰好是以下之一：" + strings.Join(spec.Params.Tags, "、") + "。"
		}
		instr += "不要输出任何多余文字。"
		return instr
	}

	// L1 (unchanged, byte-identical to before this level parameter existed).
	instr := "你是一名批判性思维教练。学生正在读下面这份材料。请针对「" + spec.Name +
		"」的每个维度，在材料里挑出一处最相关的原句，提出一个指向那句话的具体引导问题。\n" +
		"维度：\n" + dims.String() +
		"\n只输出 JSON 数组，每个元素形如 {\"block_id\":\"m0:b0\",\"quote\":\"材料里的原句片段\",\"dimension\":\"维度名\",\"question\":\"你的问题\"}。"
	if tagged {
		instr += "dimension 字段必须恰好是以下之一：" + strings.Join(spec.Params.Tags, "、") + "。"
	}
	instr += "block_id 必须来自材料，quote 必须是该 block 里的原文片段。不要输出任何多余文字。"
	return instr
}

// errString is a tiny error helper (avoids importing errors just for this file).
type errString string

func (e errString) Error() string { return string(e) }
