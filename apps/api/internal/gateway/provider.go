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
	Provider               string // "deepseek" | "anthropic" | "glm"
	BaseURL                string
	Model                  string
	APIKey                 string
	Tier                   string // "chaperone" (chat) — eval tier is P1.3
	DefaultReasoningEffort string // model-profile default; request value takes precedence
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
