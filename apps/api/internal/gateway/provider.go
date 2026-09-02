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
	// EventReasoningDelta carries the model's thinking, kept on its own kind so
	// it can never leak into a reply body by accident. A consumer that does not
	// know about it ignores it — which is what every consumer written before
	// 2026-09-02 does, and why adding this changed no behaviour.
	//
	// Thinking is NOT persisted. It is shown, folded, for the turn it belongs
	// to and then it is gone: it is the model's scratch work, not the student's
	// record, and the process tree is the student's record.
	EventReasoningDelta
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

// wantsThinkingOff decides whether this call must not reason. The capability
// class is the authority: reflex, dialogue and digest say "off"; review and
// assess do not. Tier remains the fallback for a Resolved built without a class
// — hand-built values in tests, and ResolveDirect from the offline workbench —
// so this change is invisible to every existing caller.
// Precedence, most specific first:
//
//	req.DisableThinking  — this call must not think, whatever the class says
//	req.ReasoningEffort  — this call asked for a BUDGET, so it must think a little
//	r.Reasoning          — the capability class's requirement
//	r.Tier               — pre-class fallback (chaperone ⇒ off)
//
// The second rule is load-bearing. agent.RouteReading asks for "low" and its own
// comment records why: thinking-OFF breaks that router outright (empty replies,
// stops offering cards), while full thinking costs 19–49s a turn. A class default
// of "off" silently overruling that request would have re-broken the reading room
// the same way, and nothing but a live read-together turn would have shown it.
func wantsThinkingOff(r Resolved, req ChatRequest) bool {
	if req.DisableThinking {
		return true
	}
	if req.ReasoningEffort != "" {
		return false
	}
	switch r.Reasoning {
	case ReasoningOff:
		return true
	case "":
		return r.Tier == "chaperone"
	}
	return false
}

// Resolved is the per-call resolution of which provider/model/key to use. It is
// built by a KeyResolver from server-side config. APIKey is a secret and must
// never be logged or echoed.
type Resolved struct {
	Provider string // catalog provider id — "dashscope" | "deepseek" | "anthropic" | "glm"
	// Kind is the wire protocol the adapter must speak ("openai_compatible" |
	// "anthropic"). The mux dispatches on it, which is why an OpenAI-compatible
	// vendor is a catalog entry rather than a new Go adapter. Empty falls back to
	// dispatching on Provider, keeping pre-catalog Resolved values working.
	Kind    string
	BaseURL string
	Model   string // the name put on the wire
	// ModelID is the catalog id ("dashscope/qwen3.8-max") — for logs and --print-models.
	ModelID string
	APIKey  string
	Tier    string // "chaperone" | "flagship"
	// Reasoning is the capability class's requirement — "off" | "low" | "high" |
	// "max" | "default". It is the authority on whether this call may think.
	// Empty means the class had no opinion, and the adapters fall back to the
	// older Tier rule (chaperone ⇒ off), which is what keeps hand-built Resolved
	// values in tests behaving as they did.
	Reasoning              string
	DefaultReasoningEffort string // class requirement, else model-profile default; request value takes precedence
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
