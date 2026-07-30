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

// SequenceStubProvider replays a DIFFERENT script per Stream call, in order (the
// last script repeats once exhausted). Test double for retry paths, where the
// first attempt must differ from the second. Test double only.
type SequenceStubProvider struct {
	scripts     [][]StreamEvent
	Calls       int
	LastRequest ChatRequest
}

// NewSequenceStubProvider returns a provider that emits scripts[0] on the first
// call, scripts[1] on the second, and so on (clamping to the last script).
func NewSequenceStubProvider(scripts ...[]StreamEvent) *SequenceStubProvider {
	return &SequenceStubProvider{scripts: scripts}
}

func (s *SequenceStubProvider) Stream(ctx context.Context, _ Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	s.LastRequest = req
	i := s.Calls
	if i >= len(s.scripts) {
		i = len(s.scripts) - 1
	}
	s.Calls++
	script := s.scripts[i]
	out := make(chan StreamEvent, len(script))
	go func() {
		defer close(out)
		for _, ev := range script {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}
