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

// Stream routes to the registered adapter. An unknown provider is a client-safe
// errStreamFailed (no secret); the caller logs and surfaces a generic error.
func (m *MuxProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	p, ok := m.providers[r.Provider]
	if !ok {
		return nil, fmt.Errorf("%w: unknown provider %q", errStreamFailed, r.Provider)
	}
	return p.Stream(ctx, r, req)
}
