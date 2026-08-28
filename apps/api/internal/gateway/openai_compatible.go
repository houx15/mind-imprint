package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAISerializeMessage maps the shared chat contract to the OpenAI-compatible
// wire format used by DeepSeek and GLM.
func openAISerializeMessage(m ChatMessage) map[string]any {
	if m.Role == RoleTool {
		return map[string]any{"role": "tool", "tool_call_id": m.ToolCallID, "content": m.Content}
	}
	if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
		calls := make([]map[string]any, len(m.ToolCalls))
		for i, c := range m.ToolCalls {
			args, _ := json.Marshal(c.Args)
			calls[i] = map[string]any{
				"id":       c.ID,
				"type":     "function",
				"function": map[string]any{"name": c.Name, "arguments": string(args)},
			}
		}
		var content any
		if m.Content == "" {
			content = nil
		} else {
			content = m.Content
		}
		return map[string]any{"role": "assistant", "content": content, "tool_calls": calls}
	}
	return map[string]any{"role": m.Role, "content": m.Content}
}

// buildOpenAICompatibleBody supplies the request fields shared by compatible
// providers. Provider-specific reasoning policy is added by each adapter.
func buildOpenAICompatibleBody(r Resolved, req ChatRequest, defaultMaxTokens int) map[string]any {
	msgs := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAISerializeMessage(m)
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	body := map[string]any{
		"model":      r.Model,
		"messages":   msgs,
		"max_tokens": maxTokens,
		"stream":     true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}
	if req.ResponseFormat == ResponseFormatJSONObject {
		body["response_format"] = map[string]any{"type": "json_object"}
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"type":     "function",
				"function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Parameters},
			}
		}
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	return body
}

func streamOpenAICompatible(ctx context.Context, client *http.Client, r Resolved, body map[string]any, provider string) (<-chan StreamEvent, error) {
	payload, _ := json.Marshal(body)
	endpoint := strings.TrimRight(r.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, errStreamFailed
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errStreamFailed
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: %s http %d", errStreamFailed, provider, resp.StatusCode)
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		consumeOpenAICompatible(ctx, resp.Body, out)
	}()
	return out, nil
}

// openAIChunk is one OpenAI-compatible streamed delta frame. reasoning_content
// is intentionally omitted: the gateway never emits or persists chain-of-thought.
type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		CompletionTokensDetails *struct {
			ReasoningTokens *int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func mapOpenAIFinish(reason string) StopReason {
	switch reason {
	case "tool_calls":
		return StopToolCall
	case "stop":
		return StopStop
	case "length":
		return StopLength
	default:
		return StopOther
	}
}

// consumeOpenAICompatible parses an OpenAI-compatible SSE stream, accumulating
// tool-call argument fragments by index before emitting a complete tool_use.
func consumeOpenAICompatible(ctx context.Context, body io.Reader, out chan<- StreamEvent) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type acc struct {
		id   string
		name string
		args strings.Builder
	}
	tools := map[int]*acc{}
	var order []int
	var stop StopReason
	completed := false

	emit := func(ev StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- ev:
			return true
		}
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			completed = true
			break
		}
		var chunk openAIChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage := &ChatUsage{InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens}
			if details := chunk.Usage.CompletionTokensDetails; details != nil && details.ReasoningTokens != nil {
				reasoning := *details.ReasoningTokens
				usage.ReasoningTokens = &reasoning
			}
			if !emit(StreamEvent{Kind: EventUsage, Usage: usage}) {
				return
			}
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" && !emit(StreamEvent{Kind: EventTextDelta, TextDelta: ch.Delta.Content}) {
				return
			}
			for _, tc := range ch.Delta.ToolCalls {
				a := tools[tc.Index]
				if a == nil {
					a = &acc{}
					tools[tc.Index] = a
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					a.id = tc.ID
				}
				if tc.Function.Name != "" {
					a.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					a.args.WriteString(tc.Function.Arguments)
				}
			}
			if ch.FinishReason != nil {
				stop = mapOpenAIFinish(*ch.FinishReason)
			}
		}
	}

	if err := sc.Err(); err != nil {
		emit(StreamEvent{Kind: EventDone, StopReason: StopOther, Incomplete: true})
		return
	}
	for _, idx := range order {
		a := tools[idx]
		if a.name == "" {
			continue
		}
		if !emit(StreamEvent{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: a.id, Name: a.name, ArgsJSON: a.args.String()}}) {
			return
		}
	}
	emit(StreamEvent{Kind: EventDone, StopReason: stop, Incomplete: !completed})
}
