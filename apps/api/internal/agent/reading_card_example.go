package agent

// reading_card_example.go — the lens-library summon path: given a card the
// STUDENT chose herself (as opposed to readturn.go's router-chosen summon),
// generate ONE example anchor so the pick-your-evidence flow has something to
// hang on. One LLM call (flagship resolver), same verbatim-quote integrity
// rule as ResolveExampleAnchor (reading_gate.go): a (0,0)/non-verbatim quote
// is never turned into an anchor — the caller degrades to a coach message
// instead of a broken card.

import (
	"context"
	"encoding/json"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// cardExampleReply is the model's raw JSON contract for ProposeCardExample.
type cardExampleReply struct {
	BlockID string `json:"block_id"`
	Quote   string `json:"quote"`
	Why     string `json:"why"`
}

// ProposeCardExample asks the flagship model to pick ONE sentence from the
// article that best illustrates the given lens (spec), plus a short why, then
// validates the quote is verbatim before minting an L1 example anchor. Always
// returns the resolved/usage of any real call attempted, even on failure
// after the call succeeded (parse error, non-verbatim quote) — the caller
// must record cost whenever Resolved.Provider != "" regardless of ok. ok is
// false on any failure: resolver error, Collect error, parse error, or a
// quote that is not a verbatim substring of the named block. Never returns a
// (0,0) anchor.
func ProposeCardExample(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, spec cards.Spec, materialID string, blocks []MaterialBlock) (Anchor, gateway.Resolved, gateway.ChatUsage, bool) {
	resolved, err := resolver(ctx)
	if err != nil {
		return Anchor{}, gateway.Resolved{}, gateway.ChatUsage{}, false
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildCardExamplePrompt(spec)},
			{Role: gateway.RoleUser, Content: renderCardExampleArticle(blocks)},
		},
		// 3000, not 400: reasoning-model headroom (see reading_router.go) — a
		// truncated example anchor makes the card summon degrade/fail.
		MaxTokens: 3000,
	}
	// Up to 2 attempts, mirroring RouteReading: the flagship is a reasoning model
	// that intermittently returns a truncated/unparseable reply or a
	// non-verbatim quote on a real article (~1 in 4 summons in a live sweep),
	// which lands the student on a no-example card. One retry cuts that tail;
	// usage accumulates across attempts so 档位+token+成本 stays accurate. The
	// resolved/usage are attached from the first real call on, since it cost
	// money regardless of what parsing does next.
	var total gateway.ChatUsage
	for attempt := 0; attempt < 2; attempt++ {
		res, cerr := gateway.Collect(ctx, p, resolved, req)
		if cerr != nil {
			return Anchor{}, resolved, total, false // network error: don't hammer the API
		}
		total.InputTokens += res.Usage.InputTokens
		total.OutputTokens += res.Usage.OutputTokens
		var reply cardExampleReply
		if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil {
			continue // truncated/garbage — retry once, then degrade
		}
		var text string
		found := false
		for _, b := range blocks {
			if b.ID == reply.BlockID {
				text, found = b.Text, true
				break
			}
		}
		if !found {
			continue
		}
		start, end := computeOffsets(text, reply.Quote)
		if end <= start { // (0,0) means "not a substring" — retry, then reject.
			continue
		}
		why := reply.Why
		if why == "" {
			why = "先看这处示范，再换你在文章里找一句自己的证据。"
		}
		return Anchor{
			ID: "ex0", MaterialID: materialID, BlockID: reply.BlockID,
			Start: start, End: end, Quote: reply.Quote,
			Dimension: spec.ID, Author: "ai", Question: why,
		}, resolved, total, true
	}
	return Anchor{}, resolved, total, false
}

func buildCardExamplePrompt(spec cards.Spec) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练。学生自己选了一副思维透镜「" + spec.Name + "」，想看看它能怎么用在这篇文章上。\n")
	if lens := spec.ReadingLens; lens != nil {
		// A lens is purpose-built for the pick-one-sentence mechanic — steer the
		// model with its own task/hint/focus so it reliably finds a groundable
		// single sentence (the whole reason lenses replaced the tool cards here).
		b.WriteString("这副透镜是做什么的：" + spec.Purpose + "\n")
		b.WriteString("要挑什么样的句子：" + lens.TaskPrompt + "\n")
		b.WriteString("怎么找：" + lens.SelectionHint + "\n")
		b.WriteString("示范要突出什么：" + lens.ExampleFocus + "\n")
	} else {
		what := spec.Purpose
		if spec.TriggerCondition != "" {
			what = spec.TriggerCondition + "。" + what
		}
		b.WriteString("这副透镜是做什么的：" + what + "\n")
	}
	b.WriteString("请从文章里挑出恰好一句最能示范这副透镜的原句，并用不超过两句话解释为什么这句适合——克制、贴合原文，不要替她下最终结论，只是给她一个示范起点。\n")
	b.WriteString("只输出 JSON：{\"block_id\":\"示范句所在的 block id\",\"quote\":\"该 block 里的一句原文，必须逐字来自原文\",\"why\":\"不超过两句话的中文解释\"}。不要输出任何多余文字。")
	return b.String()
}

// renderCardExampleArticle renders the article the same block-labelled shape
// BuildMaterialContext uses ("[block_id] text"), so the model's block_id
// reply round-trips directly against blocks without any alias translation —
// this path has exactly one material, unlike BuildMaterialContext's
// multi-material "mN:blockID" qualifying.
func renderCardExampleArticle(blocks []MaterialBlock) string {
	var b strings.Builder
	b.WriteString("文章：\n")
	for _, blk := range blocks {
		b.WriteString("[" + blk.ID + "] " + blk.Text + "\n")
	}
	return b.String()
}
