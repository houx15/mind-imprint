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

// DeepSeekProvider streams from an OpenAI-compatible /chat/completions endpoint
// (stream:true). Request field-mapping and tool format match the TS openaiAdapter.
type DeepSeekProvider struct {
	http *http.Client
}

// NewDeepSeekProvider returns a DeepSeekProvider using the given HTTP client.
func NewDeepSeekProvider(c *http.Client) *DeepSeekProvider {
	if c == nil {
		c = http.DefaultClient
	}
	return &DeepSeekProvider{http: c}
}

// openaiSerializeMessage mirrors the TS openaiAdapter serializeMessage.
func openaiSerializeMessage(m ChatMessage) map[string]any {
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

func (p *DeepSeekProvider) buildBody(r Resolved, req ChatRequest) map[string]any {
	msgs := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiSerializeMessage(m)
	}
	// max_tokens is an upper BOUND, not a target — a short reply still stops
	// early, so a generous default costs nothing for small calls but prevents
	// large structured outputs (assessment report, weekly/parent prose,
	// whole-draft review) from being truncated mid-JSON. The old 1024 default
	// silently truncated every unset large-output call (empty/partial content →
	// "unexpected end of JSON input" → 422). Call sites that WANT a tight cap
	// still set MaxTokens explicitly (e.g. course render 1200).
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		// 16000 (verified accepted by deepseek-v4-pro): the large-output calls
		// that rely on this default (mirror / weekly / parent prose / whole-draft
		// review) were, like the assessment report, truncating mid-JSON at 8000
		// on rich inputs — a reasoning model spends part of the budget on
		// reasoning tokens, so the visible-output headroom must be generous.
		maxTokens = 16000
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
	// Chaperone tier turns OFF v4 "thinking" for speed (陪练走中档模型可降级) —
	// ~2-3× faster. Prompt tuning alone couldn't make a non-reasoning model drive
	// the coach's mechanical transitions (set_status / generate_plan), so those
	// moved server-side (reconcileStudioFunnel): the funnel now advances the stage
	// and auto-generates the plan deterministically in Go, leaving the model only
	// propose_note + narrate — which it does reliably fast. Flagship tier keeps
	// thinking ON (omits the param) for evaluation depth (评估走旗舰模型绝不降级);
	// empty tier keeps it on. Anthropic ignores this.
	if r.Tier == "chaperone" {
		body["thinking"] = map[string]any{"type": "disabled"}
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

// Stream issues the streaming request and emits StreamEvents.
func (p *DeepSeekProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	body, _ := json.Marshal(p.buildBody(r, req))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, errStreamFailed
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, errStreamFailed
	}
	if resp.StatusCode != http.StatusOK {
		// Drain + close; surface a redacted error (status only, never the body).
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: deepseek http %d", errStreamFailed, resp.StatusCode)
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		p.consume(ctx, resp.Body, out)
	}()
	return out, nil
}

// openaiChunk is one streamed delta frame.
type openaiChunk struct {
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
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
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

// consume parses the OpenAI-compatible SSE, accumulating tool-call argument
// fragments by index, then emits the reassembled tool_use before Done.
func (p *DeepSeekProvider) consume(ctx context.Context, body io.Reader, out chan<- StreamEvent) {
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
			break
		}
		var chunk openaiChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			if !emit(StreamEvent{Kind: EventUsage, Usage: &ChatUsage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}}) {
				return
			}
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				if !emit(StreamEvent{Kind: EventTextDelta, TextDelta: ch.Delta.Content}) {
					return
				}
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

	// Surface scanner errors (e.g. line exceeding 1 MB buffer) rather than
	// silently falling through to a fake-success Done.
	if err := sc.Err(); err != nil {
		emit(StreamEvent{Kind: EventDone, StopReason: StopOther})
		return
	}

	// Emit reassembled tool calls (in first-seen index order) before Done.
	for _, idx := range order {
		a := tools[idx]
		if a.name == "" {
			continue
		}
		if !emit(StreamEvent{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: a.id, Name: a.name, ArgsJSON: a.args.String()}}) {
			return
		}
	}
	emit(StreamEvent{Kind: EventDone, StopReason: stop})
}
