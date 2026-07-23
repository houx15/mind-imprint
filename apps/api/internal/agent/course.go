package agent

import (
	"context"
	"encoding/json"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/materialize"
)

type CourseAsset struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Value string `json:"value"`
}

type CourseStepInput struct {
	Ordinal         int32
	Kind            string
	Purpose         string
	Assets          []CourseAsset
	ChallengeType   *string
	AuthoredContent json.RawMessage
}

type RenderedStep struct {
	Ordinal  int32           `json:"ordinal"`
	Kind     string          `json:"kind"`
	Template string          `json:"template"`
	Content  json.RawMessage `json:"content"`
	Source   string          `json:"source"`

	// Resolved/Usage carry the LLM call's routing + token usage for the
	// caller to record as an llm_call row (purpose="course_render", design's
	// "记录档位 + token + 成本" hard constraint) — zero-value when
	// authoredStep's fallback ran (no real call was made). Never serialized:
	// internal cost/usage detail must not leak to the student's browser.
	Resolved gateway.Resolved  `json:"-"`
	Usage    gateway.ChatUsage `json:"-"`
}

func authoredStep(in CourseStepInput, template string) RenderedStep {
	c := in.AuthoredContent
	if len(c) == 0 {
		c = json.RawMessage("{}")
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: template, Content: c, Source: "authored"}
}

// RenderCourseStep produces a step's content within a fixed template, generating
// server-side with a mandatory authored fallback so a step always renders.
func RenderCourseStep(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	if in.Kind == "challenge" {
		return renderChallenge(ctx, in, provider, resolver)
	}
	return renderTeaching(ctx, in, provider, resolver)
}

func renderTeaching(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	authored := authoredStep(in, "teaching")
	resolved, err := resolver(ctx)
	if err != nil {
		return authored
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: teachingPrompt(in)},
			{Role: gateway.RoleUser, Content: assetsText(in.Assets)},
		},
		MaxTokens: 1200,
	}
	res, err := gateway.Collect(ctx, provider, resolved, req)
	if err != nil {
		return authored
	}
	// A real call succeeded — it cost money regardless of what parsing does
	// next, so the fallback we might still return below carries usage too.
	authored.Resolved = resolved
	authored.Usage = res.Usage
	var tc struct {
		Title             string   `json:"title"`
		Subtitle          string   `json:"subtitle"`
		Body              []string `json:"body"`
		ForegroundAssetID *string  `json:"foreground_asset_id"`
	}
	if json.Unmarshal([]byte(stripFences(res.Text)), &tc) != nil || tc.Title == "" || tc.Subtitle == "" || len(tc.Body) == 0 {
		return authored
	}
	raw, err := json.Marshal(tc)
	if err != nil {
		return authored
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: "teaching", Content: raw, Source: "generated", Resolved: resolved, Usage: res.Usage}
}

func renderChallenge(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	authored := authoredStep(in, "challenge")
	var frame struct {
		Title      string `json:"title"`
		Prompt     string `json:"prompt"`
		ReasonHint string `json:"reason_hint"`
	}
	_ = json.Unmarshal(in.AuthoredContent, &frame)

	var text string
	for _, a := range in.Assets {
		if a.Kind == "text" {
			text = a.Value
			break
		}
	}
	if text == "" {
		return authored
	}
	blocks := materialize.Segment(text)
	mBlocks := make([]MaterialBlock, 0, len(blocks))
	for _, b := range blocks {
		mBlocks = append(mBlocks, MaterialBlock{ID: b.ID, Text: b.Text})
	}
	materials := []Material{{ID: "m0", Title: frame.Title, Blocks: mBlocks}}
	spec := cards.Spec{Name: frame.Title, Steps: challengeDimensions(in.ChallengeType)}

	gen, err := NewAnchorGenerator(provider, resolver).Generate(ctx, spec, materials, GuidanceL1)
	// Carry usage onto the fallback BEFORE any empty-result bail — a populated
	// Resolved means a paid call happened regardless of anchored-ness (N6 C5).
	if gen.Resolved.Provider != "" {
		authored.Resolved = gen.Resolved
		authored.Usage = gen.Usage
	}
	if err != nil || len(gen.Anchors) == 0 {
		return authored
	}
	// AnchorGenerator substitutes generic unanchored fallback anchors (empty
	// BlockID) when the model fails; that means real generation didn't happen —
	// prefer the curated authored challenge instead of generic placeholders.
	anchored := false
	for _, a := range gen.Anchors {
		if a.BlockID != "" {
			anchored = true
			break
		}
	}
	if !anchored {
		return authored
	}
	content := map[string]any{"title": frame.Title, "prompt": frame.Prompt, "anchors": gen.Anchors, "reason_hint": frame.ReasonHint}
	raw, err := json.Marshal(content)
	if err != nil {
		return authored
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: "challenge", Content: raw, Source: "generated", Resolved: gen.Resolved, Usage: gen.Usage}
}

func teachingPrompt(in CourseStepInput) string {
	return "你是一名循循善诱的老师。这一步的教学目的是：" + in.Purpose +
		"。请基于下面的素材，写一小段讲解。\n" +
		"只输出 JSON：{\"title\":\"这页的小标题\",\"subtitle\":\"一句话的口播导语\",\"body\":[\"第一段\",\"第二段\"],\"foreground_asset_id\":\"要重点展示的素材 id 或 null\"}。不要输出任何多余文字。"
}

func assetsText(assets []CourseAsset) string {
	out := "# 素材\n"
	for _, a := range assets {
		out += "[" + a.ID + " · " + a.Kind + "] " + a.Title + "：" + a.Value + "\n"
	}
	return out
}

func challengeDimensions(challengeType *string) []cards.Step {
	if challengeType != nil && *challengeType == "verify_claim" {
		return []cards.Step{
			{Title: "权威性 · Authority"},
			{Title: "准确性 · Accuracy"},
			{Title: "目的性 · Purpose"},
		}
	}
	return []cards.Step{{Title: "仔细看这段材料"}}
}
