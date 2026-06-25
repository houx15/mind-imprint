package gateway

import "context"

// StubProvider replays a fixed script of StreamEvents. Test double only.
type StubProvider struct {
	script []StreamEvent
	// LastRequest captures the request the engine built, for assertions.
	LastRequest ChatRequest
}

// NewStubProvider returns a StubProvider that will emit script verbatim.
func NewStubProvider(script []StreamEvent) *StubProvider {
	return &StubProvider{script: script}
}

// Stream emits the scripted events on a buffered channel, stopping early if ctx
// is cancelled.
func (s *StubProvider) Stream(ctx context.Context, _ Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	s.LastRequest = req
	out := make(chan StreamEvent, len(s.script))
	go func() {
		defer close(out)
		for _, ev := range s.script {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}
