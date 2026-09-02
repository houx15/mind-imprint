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

	// Reasoning off for the chaperone tier (coach, guides, compaction, classify,
	// search-guidance) and for any call that asks explicitly. The flagship
	// reviewer/eval seam keeps reasoning on.
	wantOff := r.Tier == "chaperone" || req.DisableThinking
	switch {
	case !wantOff:
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
			body[pol.ReasoningEffortKey] = effort
		}
	}
	return body, nil
}

// Stream issues the streaming request and emits StreamEvents. Reasoning content
// is never surfaced or persisted.
func (p *CatalogProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
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
