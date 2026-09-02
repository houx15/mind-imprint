// Package gateway is the server-side LLM gateway: chat types mirroring the TS
// llm contract, streaming provider adapters (DeepSeek/Anthropic), the
// key-resolver seam, and the SSE writer. Secrets enter only through Resolved
// values built by a KeyResolver from server-side config; they never appear in
// logs, errors, or committed files.
package gateway

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
