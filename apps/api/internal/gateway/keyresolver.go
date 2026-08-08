package gateway

import (
	"context"
	"errors"

	"mindimprint/api/internal/config"
)

// KeyResolver resolves, per request, which provider/model/key to use. Today it
// returns the platform env secret (DeepSeek preferred, Anthropic optional). The
// seam exists so future per-org billing can resolve the caller's school key
// without changing the rest of the gateway.
type KeyResolver func(ctx context.Context) (Resolved, error)

// errNoProvider is the client-safe configuration error (no secret).
var errNoProvider = errors.New("no LLM provider configured")

// NewKeyResolver builds a platform-env resolver. DeepSeek is the default chat
// provider; Anthropic is used only when DeepSeek is absent.
//
// The chaperone tier runs deepseek-v4-pro. We measured deepseek-v4-flash here
// (cheaper, non-reasoning) hoping to cut coach latency — but it was a NET
// REGRESSION on the orchestrator's JSON-envelope turn: without the reasoning
// model's "reason silently, emit compact JSON" discipline, flash poured volume
// into the visible output (4,000–7,000 completion tokens/turn vs v4-pro's
// 600–1,100), pushing turns to 40–66s (vs ~20s) and, on the worst runaway,
// truncating the envelope mid-JSON so it fell back to the canned line. v4-pro
// is both faster in wall-clock AND better-formed for this task, so the
// chaperone stays on it. The real latency lever is SSE-streaming the narrate
// (perceived first-token latency), not the model tier.
func NewKeyResolver(cfg config.Config) KeyResolver {
	return func(_ context.Context) (Resolved, error) {
		switch {
		case cfg.DeepSeekKey != "":
			return Resolved{
				Provider: "deepseek",
				BaseURL:  "https://api.deepseek.com/v1",
				Model:    "deepseek-v4-pro",
				APIKey:   cfg.DeepSeekKey,
				Tier:     "chaperone",
			}, nil
		case cfg.AnthropicKey != "":
			return Resolved{
				Provider: "anthropic",
				BaseURL:  "https://api.anthropic.com/v1",
				Model:    "claude-3-5-sonnet-latest",
				APIKey:   cfg.AnthropicKey,
				Tier:     "chaperone",
			}, nil
		default:
			return Resolved{}, errNoProvider
		}
	}
}

// NewEvalKeyResolver builds the FLAGSHIP resolver for evaluation — never
// downgraded (评估走旗舰模型绝不降级). DeepSeek's v4-pro is the China-first
// default; Anthropic is the fallback. Same seam shape as NewKeyResolver.
func NewEvalKeyResolver(cfg config.Config) KeyResolver {
	return func(_ context.Context) (Resolved, error) {
		switch {
		case cfg.DeepSeekKey != "":
			return Resolved{
				Provider: "deepseek",
				BaseURL:  "https://api.deepseek.com/v1",
				Model:    "deepseek-v4-pro",
				APIKey:   cfg.DeepSeekKey,
				Tier:     "flagship",
			}, nil
		case cfg.AnthropicKey != "":
			return Resolved{
				Provider: "anthropic",
				BaseURL:  "https://api.anthropic.com/v1",
				Model:    "claude-3-5-sonnet-latest",
				APIKey:   cfg.AnthropicKey,
				Tier:     "flagship",
			}, nil
		default:
			return Resolved{}, errNoProvider
		}
	}
}
