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
// The chaperone tier runs deepseek-v4-flash, NOT the flagship v4-pro (陪练走中
// 档模型可降级). v4-pro is a reasoning model: it spends most of each turn on
// hidden reasoning tokens, so the coach loop (which is buffered, one call per
// student turn, plus an inline compaction call) ran ~20s/turn and climbed as
// the history window filled. Flash is a non-reasoning model — far faster and
// ~1/3 the cost — which is the right trade for the always-on chaperone. The
// orchestrator's JSON envelope is defended by ParseOrchestratorOutput's
// balanced-object salvage + the plain-prose fallback, so a less format-strict
// model degrades gracefully. Evaluation stays on the flagship (NewEvalKeyResolver,
// 评估走旗舰模型绝不降级).
func NewKeyResolver(cfg config.Config) KeyResolver {
	return func(_ context.Context) (Resolved, error) {
		switch {
		case cfg.DeepSeekKey != "":
			return Resolved{
				Provider: "deepseek",
				BaseURL:  "https://api.deepseek.com/v1",
				Model:    "deepseek-v4-flash",
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
