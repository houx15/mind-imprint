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
		// The cached share rides along, or the second attempt — which repeats a
		// prompt the provider has just seen, so it is almost entirely a cache
		// hit — gets billed at list price in our own records.
		total.CachedInputTokens += res.Usage.CachedInputTokens
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

// renderCardExampleArticle renders the article the same block-labelled shape
// BuildMaterialContext uses ("[block_id] text"), so the model's block_id
// reply round-trips directly against blocks without any alias translation —
// this path has exactly one material, unlike BuildMaterialContext's
// multi-material "mN:blockID" qualifying.
// cardExampleArticleRuneBudget caps the article this prompt carries, matching
// the budget the reading plan and the anchored questions already apply to the
// same body. Without it this was the one path that sent an article of any
// length — and it is reached twice per lens summon, each of which retries
// twice, so an unbudgeted article here is billed up to four times over.
//
// A paragraph past the budget is named rather than dropped: the model is
// picking ONE illustrative sentence, and a paragraph it cannot see is better
// declared missing than silently absent from a body it is told is complete.
const cardExampleArticleRuneBudget = 9000
