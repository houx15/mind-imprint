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

// AnthropicProvider streams from /messages (stream:true). Request field-mapping
// and tool format match the TS anthropicAdapter; it consumes the SSE event
// stream (content_block_delta / message_delta) rather than a single response.
type AnthropicProvider struct {
	http *http.Client
}

// NewAnthropicProvider returns an AnthropicProvider using the given HTTP client.
func NewAnthropicProvider(c *http.Client) *AnthropicProvider {
	if c == nil {
		c = http.DefaultClient
	}
	return &AnthropicProvider{http: c}
}

// anthropicSerializeMessage mirrors the TS anthropicAdapter serializeMessage.
func anthropicSerializeMessage(m ChatMessage) map[string]any {
	if m.Role == RoleTool {
		return map[string]any{
			"role":    "user",
			"content": []any{map[string]any{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content}},
		}
	}
	if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
		var content []any
		if m.Content != "" {
			content = append(content, map[string]any{"type": "text", "text": m.Content})
		}
		for _, c := range m.ToolCalls {
			content = append(content, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": c.Args})
		}
		return map[string]any{"role": "assistant", "content": content}
	}
	return map[string]any{"role": m.Role, "content": m.Content}
}

func (p *AnthropicProvider) buildBody(r Resolved, req ChatRequest) map[string]any {
	var systemParts []string
	var msgs []map[string]any
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			systemParts = append(systemParts, m.Content)
			continue
		}
		msgs = append(msgs, anthropicSerializeMessage(m))
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}
	body := map[string]any{
		"model":      r.Model,
		"max_tokens": maxTokens,
		"stream":     true,
		"messages":   msgs,
	}
	if s := strings.Join(systemParts, "\n\n"); s != "" {
		body["system"] = s
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Parameters}
		}
		body["tools"] = tools
	}
	return body
}

// Stream issues the streaming request and emits StreamEvents.
func (p *AnthropicProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	body, _ := json.Marshal(p.buildBody(r, req))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, errStreamFailed
	}
	httpReq.Header.Set("x-api-key", r.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, errStreamFailed
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: anthropic http %d", errStreamFailed, resp.StatusCode)
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		p.consume(ctx, resp.Body, out)
	}()
	return out, nil
}

type anthropicFrame struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func mapAnthropicStop(reason string) StopReason {
	switch reason {
	case "tool_use":
		return StopToolCall
	case "end_turn":
		return StopStop
	case "max_tokens":
		return StopLength
	default:
		return StopOther
	}
}

// consume parses the Anthropic SSE: text_delta → TextDelta; a tool_use block's
// input_json_delta fragments accumulate per index and emit a ToolUse at its
// content_block_stop; message_delta carries stop_reason + output_tokens.
func (p *AnthropicProvider) consume(ctx context.Context, body io.Reader, out chan<- StreamEvent) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type acc struct {
		id     string
		name   string
		args   strings.Builder
		isTool bool
	}
	blocks := map[int]*acc{}
	inputTokens := 0
	outputTokens := 0
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
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var f anthropicFrame
		if err := json.Unmarshal([]byte(data), &f); err != nil {
			continue
		}
		switch f.Type {
		case "message_start":
			if f.Message != nil && f.Message.Usage != nil {
				inputTokens = f.Message.Usage.InputTokens
			}
		case "content_block_start":
			if f.ContentBlock != nil {
				a := &acc{}
				if f.ContentBlock.Type == "tool_use" {
					a.isTool = true
					a.id = f.ContentBlock.ID
					a.name = f.ContentBlock.Name
				}
				blocks[f.Index] = a
			}
		case "content_block_delta":
			if f.Delta == nil {
				continue
			}
			switch f.Delta.Type {
			case "text_delta":
				if !emit(StreamEvent{Kind: EventTextDelta, TextDelta: f.Delta.Text}) {
					return
				}
			case "input_json_delta":
				if a := blocks[f.Index]; a != nil {
					a.args.WriteString(f.Delta.PartialJSON)
				}
			}
		case "content_block_stop":
			if a := blocks[f.Index]; a != nil && a.isTool {
				if !emit(StreamEvent{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: a.id, Name: a.name, ArgsJSON: a.args.String()}}) {
					return
				}
			}
		case "message_delta":
			if f.Delta != nil && f.Delta.StopReason != "" {
				stop = mapAnthropicStop(f.Delta.StopReason)
			}
			if f.Usage != nil {
				outputTokens = f.Usage.OutputTokens
			}
		case "message_stop":
			// terminal
		}
	}

	// Surface scanner errors (e.g. line exceeding 1 MB buffer) rather than
	// silently falling through to a fake-success Done.
	if err := sc.Err(); err != nil {
		emit(StreamEvent{Kind: EventDone, StopReason: StopOther})
		return
	}

	if !emit(StreamEvent{Kind: EventUsage, Usage: &ChatUsage{InputTokens: inputTokens, OutputTokens: outputTokens}}) {
		return
	}
	emit(StreamEvent{Kind: EventDone, StopReason: stop})
}
