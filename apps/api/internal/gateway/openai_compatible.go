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
			// 🚨 A nil map marshals to "null", and some providers reject that
			// where an arguments object is expected. A call with no arguments
			// goes on the wire as "{}". This is reachable: decodeToolArgs hands
			// back nil for empty or unparseable arguments, and a tool loop
			// replays that same call to the model on its next round.
			args := "{}"
			if len(c.Args) > 0 {
				if b, err := json.Marshal(c.Args); err == nil {
					args = string(b)
				}
			}
			calls[i] = map[string]any{
				"id":       c.ID,
				"type":     "function",
				"function": map[string]any{"name": c.Name, "arguments": args},
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
		// 🚨 把连接层的错因带上。它不含密钥（密钥在 header 里，不在 error 里），
		// 而少了它，服务端日志里"provider stream failed"这一句分不清是连不上、
		// 被限流、还是请求被取消——2026-09-02 排查复盘挂住时就卡在这里。
		// 客户端看到的仍是固定的错误码，不受影响。
		return nil, fmt.Errorf("%w: %s transport: %v", errStreamFailed, provider, err)
	}
	if resp.StatusCode != http.StatusOK {
		detail := readErrorBody(resp.StatusCode, resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: %s http %d%s", errStreamFailed, provider, resp.StatusCode, detail)
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		consumeOpenAICompatible(ctx, resp.Body, out)
	}()
	return out, nil
}

// openAIChunk is one OpenAI-compatible streamed delta frame.
//
// reasoning_content is now read. It used to be dropped on the floor, on the
// rule that the gateway never surfaces chain-of-thought. The product owner
// asked for the opposite (2026-09-02): show the thinking, folded, the way other
// tools do. Reading it here does not decide where it goes — it is emitted as a
// distinct event kind that a caller must opt into, and no existing caller does.
type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			// ReasoningContent is what DeepSeek/Qwen/GLM/Kimi all call the
			// thinking stream on this wire format.
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
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
			// Reasoning is emitted on its own kind, never merged into the text
			// stream. Merging them would put chain-of-thought into every
			// existing caller's reply body — including the ones that persist it
			// and the ones that show it to a student mid-sentence.
			if ch.Delta.ReasoningContent != "" &&
				!emit(StreamEvent{Kind: EventReasoningDelta, TextDelta: ch.Delta.ReasoningContent}) {
				return
			}
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

// readErrorBody returns a bounded, single-line prefix of a non-200 response body
// so the error carries the provider's own words.
//
// 🚨 We used to io.Discard this, and it cost a wrong conclusion. On 2026-09-03
// routebench recorded `dashscope http 400` for glm-5.3 and I wrote the model off
// as broken on that class. The body said, all along:
//
//	{"code":"1210","message":"该模型始终思考，不支持关闭思考；请使用 low、high 或 max。"}
//
// — a documented, actionable rule about ONE request field, thrown away by us.
// AGENTS.md §界面文案 8 already required 动词+失败 plus 后台原话; this is where the
// 后台原话 has to survive to make that possible.
//
// Safe to include: the API key travels in the Authorization header, never in a
// response body. The cap keeps a verbose HTML error page out of the logs.
func readErrorBody(status int, r io.Reader) string {
	// 🚨 Never echo the body of an auth failure. 401/403 is the one case where a
	// provider plausibly quotes the credential back at us ("Incorrect API key
	// provided: sk-…"), and AGENTS.md forbids a key reaching a thrown error. It
	// is also the one case where the body adds nothing: the fix for 401 is the
	// key, and the status already said that.
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(r, 2048))
	if err != nil || len(b) == 0 {
		return ""
	}
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 500 {
		s = s[:500] + "…"
	}
	return ": " + s
}
