package gateway

import (
	"context"
	"fmt"
)

// MuxProvider dispatches a turn to the adapter matching Resolved.Provider. The
// keyResolver decides which provider/model/key to use; the mux routes to the
// concrete adapter. Adding a provider = registering one more entry, no call-site
// change.
type MuxProvider struct {
	providers map[string]Provider
}

// NewMuxProvider builds a dispatching provider keyed by Resolved.Provider value
// ("deepseek" | "anthropic" | "glm").
func NewMuxProvider(providers map[string]Provider) *MuxProvider {
	return &MuxProvider{providers: providers}
}

// Stream routes to the registered adapter. Dispatch prefers Resolved.Kind (the
// wire protocol) so every OpenAI-compatible vendor shares one adapter and a new
// vendor needs no Go; it falls back to Resolved.Provider so pre-catalog values
// still reach their named adapter. An unknown route is a client-safe
// errStreamFailed (no secret); the caller logs and surfaces a generic error.
func (m *MuxProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	if r.Kind != "" {
		if p, ok := m.providers[r.Kind]; ok {
			return p.Stream(ctx, r, req)
		}
	}
	p, ok := m.providers[r.Provider]
	if !ok {
		return nil, fmt.Errorf("%w: unknown provider %q (kind %q)", errStreamFailed, r.Provider, r.Kind)
	}
	return p.Stream(ctx, r, req)
}
