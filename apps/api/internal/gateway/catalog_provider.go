package gateway

import (
	"context"
	"fmt"
	"net/http"
)

// CatalogProvider streams from any OpenAI-compatible endpoint, taking its
// reasoning knobs and body defaults from Resolved.Policy rather than from
// per-vendor Go. It replaces the hand-written DeepSeek/GLM buildBody pair for
// every catalog-resolved call, which is what makes "new vendor = one JSON entry"
// true rather than aspirational.
type CatalogProvider struct {
	http *http.Client
}

// NewCatalogProvider returns a CatalogProvider using the given HTTP client.
func NewCatalogProvider(c *http.Client) *CatalogProvider {
	if c == nil {
		c = http.DefaultClient
	}
	return &CatalogProvider{http: c}
}

// catalogMaxTokens is an upper BOUND, not a target: a short reply still stops
// early, so a generous default costs nothing on small calls while keeping large
// structured outputs (assessment report, weekly/parent prose, whole-draft
// review) from truncating mid-JSON.
const catalogMaxTokens = 16000

func (p *CatalogProvider) buildBody(r Resolved, req ChatRequest) (map[string]any, error) {
	body := buildOpenAICompatibleBody(r, req, catalogMaxTokens)
	pol := r.Policy

	for k, v := range pol.BodyExtra {
		body[k] = v
	}
	if len(req.Tools) > 0 {
		for k, v := range pol.BodyExtraWithTools {
			body[k] = v
		}
	}
	if req.Temperature == nil && pol.DefaultTemperature != nil {
		body["temperature"] = *pol.DefaultTemperature
	}

	if pol.RequiresUserMessage {
		ensureUserMessage(body)
	}

	if req.EnableSearch {
		// A route that cannot search must say so rather than silently answering
		// from training data — a confidently stale fact is worse than an error,
		// because nothing downstream can tell it was never looked up.
		if !pol.SearchSupported {
			return nil, fmt.Errorf("%w: %s cannot search the web", errStreamFailed, r.Model)
		}
		body["enable_search"] = true
		if req.ForcedSearch {
			body["search_options"] = map[string]any{"forced_search": true}
		}
	}

	wantOff := wantsThinkingOff(r, req)
	switch {
	case !wantOff:
	case pol.ThinkingOffUnsupported && pol.MinReasoningEffort != "" && pol.ReasoningEffortKey != "":
		// The route rejects the thinking-off knob but accepts a floor effort, and
		// that floor IS off in every way a capability class cares about: measured
		// 2026-09-03, ZHIPU/GLM-5.3 at reasoning_effort:"low" returns zero
		// reasoning tokens. DashScope's own 400 tells you to do this —
		// "该模型始终思考，不支持关闭思考；请使用 low、high 或 max。"
		body[pol.ReasoningEffortKey] = effortWord(pol, pol.MinReasoningEffort)
		return body, nil
	case pol.ThinkingOffUnsupported || len(pol.ThinkingOff) == 0:
		// An explicit request to stop reasoning on a model that cannot is an
		// error, not a silently-dropped field — the caller asked for a property
		// this route cannot provide.
		if req.DisableThinking {
			return nil, fmt.Errorf("%w: %s cannot disable thinking", errStreamFailed, r.Model)
		}
		// Tier-driven only: leave the model's own default in place.
	default:
		for k, v := range pol.ThinkingOff {
			body[k] = v
		}
		// Thinking is off; a reasoning budget would be meaningless.
		return body, nil
	}

	// A bounded reasoning budget — the middle gear between full thinking and
	// thinking-off. Sent only on routes that actually honor it: PAI ignores
	// reasoning_effort for deepseek/glm, so a route may leave the key unset
	// rather than send a field that does nothing.
	if pol.ReasoningEffortKey != "" {
		effort := req.ReasoningEffort
		if effort == "" {
			effort = r.DefaultReasoningEffort
		}
		if effort != "" {
			body[pol.ReasoningEffortKey] = effortWord(pol, effort)
		}
	}
	return body, nil
}

// Stream issues the streaming request and emits StreamEvents. Reasoning content
// is never surfaced or persisted.
//
// 🚨 A route whose streaming drops its last chunk (Policy.StreamDropsTail) is
// NOT streamed — see the field's doc comment for the measurement. It is served
// by one non-streaming request delivered as a single delta, so a caller that
// asked for a stream still gets a stream-shaped answer, just not an incremental
// one. Correctness over typing-out: the alternative is every reply on that route
// silently missing its last few characters.
func (p *CatalogProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	if r.Policy.StreamDropsTail {
		return p.streamViaComplete(ctx, r, req)
	}
	body, err := p.buildBody(r, req)
	if err != nil {
		return nil, err
	}
	name := r.Provider
	if name == "" {
		name = KindOpenAICompatible
	}
	return streamOpenAICompatible(ctx, p.http, r, body, name)
}

// ensureUserMessage appends a minimal user turn when a request carries only
// system messages, for routes that reject that shape.
//
// 🚨 Do not "simplify" this by rewriting the system message into a user message.
// The system role is what makes a long instruction prompt binding on most of
// these models; demoting it to a user turn changes the answer everywhere, to fix
// a constraint that exists on exactly one route. Appending is the smaller lie.
//
// Only routes that set requiresUserMessage take this path, so every other model
// sees byte-identical requests to the ones the benchmark scored.
func ensureUserMessage(body map[string]any) {
	msgs, ok := body["messages"].([]map[string]any)
	if !ok {
		return
	}
	for _, m := range msgs {
		if m["role"] != RoleSystem {
			return
		}
	}
	body["messages"] = append(msgs, map[string]any{"role": RoleUser, "content": "请开始。"})
}

// effortWord translates our reasoning vocabulary into the word this route
// accepts. See ModelPolicy.ReasoningEffortAliases for the measured matrix and
// what assuming one shared enum cost.
func effortWord(pol ModelPolicy, effort string) string {
	if w, ok := pol.ReasoningEffortAliases[effort]; ok {
		return w
	}
	return effort
}
