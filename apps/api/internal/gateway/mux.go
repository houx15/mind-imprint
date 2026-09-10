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

// Complete 把「一次要整份回答」转给底下那个通道，前提是它做得到。
//
// 做不到就报 errNotCompletable，`Collect` 据此退回流式。为什么值得有这条路，见
// complete.go 的文件头：DashScope 那条聚合口的**流式**回复会悄悄丢掉最后一块
// 内容，而要解析 JSON 的调用因此整份作废。
func (m *MuxProvider) Complete(ctx context.Context, r Resolved, req ChatRequest) (ChatResult, error) {
	p := m.pick(r)
	if p == nil {
		return ChatResult{}, fmt.Errorf("%w: unknown provider %q (kind %q)", errStreamFailed, r.Provider, r.Kind)
	}
	c, ok := p.(Completer)
	if !ok {
		return ChatResult{}, errNotCompletable
	}
	return c.Complete(ctx, r, req)
}

// pick 是 Stream 与 Complete 共用的那一段路由。
func (m *MuxProvider) pick(r Resolved) Provider {
	if r.Kind != "" {
		if p, ok := m.providers[r.Kind]; ok {
			return p
		}
	}
	return m.providers[r.Provider]
}
