package gateway

import (
	"context"
	"errors"
)

// EventKind enumerates the kinds of streamed events a Provider emits.
type EventKind int

const (
	EventTextDelta EventKind = iota
	EventToolUse
	EventUsage
	EventDone
)

// StreamToolUse is a fully-reassembled tool call. ArgsJSON is the raw JSON
// object string the provider streamed (OpenAI streams it as a string-delta;
// Anthropic streams it as input_json_delta fragments) — the agent layer parses it.
type StreamToolUse struct {
	ID       string
	Name     string
	ArgsJSON string
}

// StreamEvent is one item on the provider stream. Only the field matching Kind
// is populated.
type StreamEvent struct {
	Kind       EventKind
	TextDelta  string         // EventTextDelta
	ToolUse    *StreamToolUse // EventToolUse
	Usage      *ChatUsage     // EventUsage
	StopReason StopReason     // EventDone
	// Incomplete records an abnormal upstream terminal stream for observational
	// consumers. Existing callers continue to receive EventDone as before.
	Incomplete bool // EventDone
}

// Resolved is the per-call resolution of which provider/model/key to use. It is
// built by a KeyResolver from server-side config. APIKey is a secret and must
// never be logged or echoed.
type Resolved struct {
	Provider string // catalog provider id — "pai" | "deepseek" | "anthropic" | "glm"
	// Kind is the wire protocol the adapter must speak ("openai_compatible" |
	// "anthropic"). The mux dispatches on it, which is why an OpenAI-compatible
	// vendor is a catalog entry rather than a new Go adapter. Empty falls back to
	// dispatching on Provider, keeping pre-catalog Resolved values working.
	Kind    string
	BaseURL string
	Model   string // the name put on the wire
	// ModelID is the catalog id ("pai/qwen3.8-max") — for logs and --print-models.
	ModelID                string
	APIKey                 string
	Tier                   string // "chaperone" | "flagship"
	DefaultReasoningEffort string // model-profile default; request value takes precedence
	// Policy carries the route's reasoning knobs and body defaults from the
	// catalog. A zero Policy means "adapter defaults", so hand-built Resolved
	// values in tests behave as before.
	Policy ModelPolicy
}

// Provider streams a single model turn. The returned channel is closed when the
// turn ends (Done) or ctx is cancelled. Implementations bind their upstream HTTP
// call to ctx so a client disconnect cancels the provider call.
//
// EventUsage may arrive at any point before EventDone — DeepSeek emits it
// inline alongside text deltas, while Anthropic emits it post-stream after the
// final message_stop. Callers must not assume EventUsage precedes EventToolUse.
type Provider interface {
	Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error)
}

// errStreamFailed is the single client-safe error a provider surfaces. The real
// cause (status body, network error) is wrapped only for server-side logging by
// the caller; the message itself carries no secret and no upstream body.
var errStreamFailed = errors.New("provider stream failed")
