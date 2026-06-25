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
func NewKeyResolver(cfg config.Config) KeyResolver {
	return func(_ context.Context) (Resolved, error) {
		switch {
		case cfg.DeepSeekKey != "":
			return Resolved{
				Provider: "deepseek",
				BaseURL:  "https://api.deepseek.com/v1",
				Model:    "deepseek-chat",
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
