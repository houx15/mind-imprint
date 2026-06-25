package gateway

import (
	"context"
	"strings"
	"testing"
)

func TestAnthropicStreamsTextThenToolUse(t *testing.T) {
	// Anthropic streaming event sequence: message_start, a text content block,
	// a tool_use content block whose input arrives via input_json_delta, then
	// message_delta carrying stop_reason + usage, then message_stop.
	frames := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":120,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"，核查一下"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"summon_card","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"card_id\":\"sift"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"_craap\",\"reason\":\"r\",\"nudge_text\":\"n\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":45}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	srv := cannedSSE(t, frames)
	defer srv.Close()

	p := NewAnthropicProvider(srv.Client())
	ch, err := p.Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "claude-3-5-sonnet-latest", APIKey: "sk"}, ChatRequest{
		Messages: []ChatMessage{
			{Role: RoleSystem, Content: "SYS"},
			{Role: RoleUser, Content: "hi"},
		},
		Tools: []ChatTool{{Name: "summon_card", Description: "d", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	var tool *StreamToolUse
	var usage *ChatUsage
	var stop StopReason
	for ev := range ch {
		switch ev.Kind {
		case EventTextDelta:
			text.WriteString(ev.TextDelta)
		case EventToolUse:
			tool = ev.ToolUse
		case EventUsage:
			usage = ev.Usage
		case EventDone:
			stop = ev.StopReason
		}
	}
	if text.String() != "你好，核查一下" {
		t.Fatalf("text = %q", text.String())
	}
	if tool == nil || tool.Name != "summon_card" || tool.ID != "toolu_1" {
		t.Fatalf("tool use missing/wrong: %+v", tool)
	}
	if tool.ArgsJSON != `{"card_id":"sift_craap","reason":"r","nudge_text":"n"}` {
		t.Fatalf("reassembled args = %q", tool.ArgsJSON)
	}
	if usage == nil || usage.InputTokens != 120 || usage.OutputTokens != 45 {
		t.Fatalf("usage = %+v", usage)
	}
	if stop != StopToolCall {
		t.Fatalf("stop = %q", stop)
	}
}
