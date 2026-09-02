package gateway

import (
	"context"
	"net/http"
)

// DeepSeekProvider streams from an OpenAI-compatible /chat/completions endpoint
// (stream:true). Request field-mapping and tool format match the TS openaiAdapter.
//
// SUPERSEDED by CatalogProvider, which takes the same knobs from models.json
// instead of hardcoding them. Nothing in production reaches this adapter any
// more: every Resolved now comes from the catalog and carries a Kind, so the mux
// routes it to CatalogProvider. It is kept because its tests cover the shared
// transport (stream reassembly, incomplete-[DONE], non-leaking HTTP errors).
//
// Do not route new work here — a hand-built Resolved{Provider: "deepseek"} would
// get this hardcoded body rather than the catalog's policy, which is exactly the
// drift the catalog exists to end.
type DeepSeekProvider struct {
	http *http.Client
}

// NewDeepSeekProvider returns a DeepSeekProvider using the given HTTP client.
func NewDeepSeekProvider(c *http.Client) *DeepSeekProvider {
	if c == nil {
		c = http.DefaultClient
	}
	return &DeepSeekProvider{http: c}
}

func (p *DeepSeekProvider) buildBody(r Resolved, req ChatRequest) map[string]any {
	// max_tokens is an upper BOUND, not a target — a short reply still stops
	// early, so a generous default costs nothing for small calls but prevents
	// large structured outputs (assessment report, weekly/parent prose,
	// whole-draft review) from being truncated mid-JSON. The old 1024 default
	// silently truncated every unset large-output call (empty/partial content →
	// "unexpected end of JSON input" → 422). Call sites that WANT a tight cap
	// still set MaxTokens explicitly (e.g. course render 1200).
	body := buildOpenAICompatibleBody(r, req, 16000)
	// Model-routing (2026-08-10, product owner's decision): reasoning stays ON only
	// for the FLAGSHIP reviewer seam (EvalResolver: framework review / plan-gen /
	// 整稿体检 / 证据地图饱和 / 评估). The coach + guide side (chaperone tier — coach,
	// guides, compaction, classify, search-guidance) runs deepseek-v4-pro with
	// request-level thinking DISABLED: faster + cheaper. An earlier reasoning-off
	// attempt regressed the coach's propose_note (misrouted sections), but that was
	// on the mega-orchestrator; the status-router shrank each turn to a tiny prompt,
	// and v4-pro (not flash) is the base here — verified live after this change.
	if r.Tier == "chaperone" || req.DisableThinking {
		body["thinking"] = map[string]any{"type": "disabled"}
	}
	// A bounded reasoning budget — the middle gear between full thinking and
	// thinking-off. Set independently of tier (a flagship call can still ask for
	// "low"); ignored alongside thinking:disabled, which already zeroes it.
	if req.ReasoningEffort != "" {
		body["reasoning_effort"] = req.ReasoningEffort
	}
	return body
}

// Stream issues the streaming request and emits StreamEvents.
func (p *DeepSeekProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	return streamOpenAICompatible(ctx, p.http, r, p.buildBody(r, req), "deepseek")
}
