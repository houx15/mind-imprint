// Package gateway is the server-side LLM gateway: chat types mirroring the TS
// llm contract, streaming provider adapters (DeepSeek/Anthropic), the
// key-resolver seam, and the SSE writer. Secrets enter only through Resolved
// values built by a KeyResolver from server-side config; they never appear in
// logs, errors, or committed files.
package gateway

import (
	"encoding/json"
	"strings"
)

// ChatRole mirrors the TS ChatRole union.
type ChatRole = string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
	RoleTool      ChatRole = "tool"
)

// StopReason mirrors the TS StopReason union.
type StopReason = string

const (
	StopStop     StopReason = "stop"
	StopToolCall StopReason = "tool_call"
	StopLength   StopReason = "length"
	StopOther    StopReason = "other"
)

// ToolCall mirrors TS ToolCall { id, name, args }.
type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// decodeToolArgs parses the JSON object a provider sent as one tool call's
// arguments. Both paths that build a ChatResult call it — the streaming one in
// collect.go and the non-streaming one in complete.go — so a tool loop gets the
// same call whichever channel served it.
//
// Empty or malformed arguments give a nil map, which reaches the caller as a
// call with no arguments. That is a state every tool executor already has to
// handle, since a model can call a tool with `{}` as well; the executor says
// which argument is missing and the model gets another round to fix it.
func decodeToolArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// ChatTool mirrors TS ChatTool { name, description, parameters }.
type ChatTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatMessage mirrors TS ChatMessage { role, content, toolCalls?, toolCallId? }.
type ChatMessage struct {
	Role       ChatRole   `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string     `json:"toolCallId,omitempty"`
}

// ResponseFormat restricts the provider response encoding when supported.
// Empty keeps the provider's default response behavior.
type ResponseFormat = string

const (
	ResponseFormatJSONObject ResponseFormat = "json_object"
)

// ChatRequest mirrors TS ChatRequest.
type ChatRequest struct {
	Messages  []ChatMessage `json:"messages"`
	Tools     []ChatTool    `json:"tools,omitempty"`
	MaxTokens int           `json:"maxTokens,omitempty"`
	// ReasoningEffort, when set, asks a reasoning model to spend a bounded amount
	// of thinking rather than its full budget. It is the middle gear between full
	// reasoning (flagship default) and thinking fully disabled (chaperone tier):
	// the reading router uses "low" to roughly halve latency while keeping the
	// decision + JSON discipline that thinking-off loses. Providers that support
	// the field emit it; GLM otherwise defaults it to its recommended "max".
	ReasoningEffort string         `json:"reasoningEffort,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	DisableThinking bool           `json:"disableThinking,omitempty"`
	ResponseFormat  ResponseFormat `json:"responseFormat,omitempty"`
	// EnableSearch lets the model reach the live web for THIS call. Off by
	// default and opt-in per call, never per class: whether a turn needs the
	// internet is a property of the question asked, not of how much intelligence
	// the call needs.
	//
	// It costs tokens (the search results enter the prompt) and latency, so a
	// coaching turn that only needs to ask the next question must not pay for it.
	// ForcedSearch removes the model's discretion to skip searching — use it when
	// the call exists BECAUSE something must be looked up.
	//
	// Measured 2026-09-03: on this endpoint the reply cites its sources in prose
	// but the response carries no machine-readable search_results. So this is the
	// right tool for "陪练需要一个事实"; it cannot hand 溯源体检 a list of URLs to
	// CRAAP-check — that still needs internal/websearch.
	EnableSearch bool `json:"enableSearch,omitempty"`
	ForcedSearch bool `json:"forcedSearch,omitempty"`
}

// ChatUsage mirrors provider usage. ReasoningTokens is optional because only
// providers that expose a completion-token breakdown can supply it.
type ChatUsage struct {
	InputTokens     int  `json:"inputTokens"`
	OutputTokens    int  `json:"outputTokens"`
	ReasoningTokens *int `json:"reasoningTokens,omitempty"`
}

// ChatResult mirrors TS ChatResult (the accumulated, non-streamed shape; useful
// for tests and any non-streaming caller).
type ChatResult struct {
	Text string `json:"text"`
	// Reasoning is the model's thinking for this turn, when the route emitted
	// any. It is deliberately NOT part of Text: every caller that persists a
	// reply, checks it against enforcement rules, or shows it to a student
	// reads Text, and none of them should suddenly be handling chain-of-thought.
	//
	// It is not persisted anywhere. It is shown, folded, for the turn it
	// belongs to and then it is gone — the model's scratch work is not the
	// student's record; the process tree is.
	Reasoning  string     `json:"reasoning,omitempty"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	StopReason StopReason `json:"stopReason,omitempty"`
	Usage      ChatUsage  `json:"usage"`
}
