package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// cannedSSE writes the given frames as an SSE response.
func cannedSSE(t *testing.T, frames string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, frames)
	}))
}

func TestDeepSeekStreamsTextThenToolUse(t *testing.T) {
	// Two text deltas, then a tool_call streamed in two argument fragments, then done.
	frames := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"你好"}}]}`,
		`data: {"choices":[{"delta":{"content":"，核查一下"}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"summon_card","arguments":"{\"card_id\":\"cr"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"aap\",\"reason\":\"r\",\"nudge_text\":\"n\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":120,"completion_tokens":45}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")

	srv := cannedSSE(t, frames)
	defer srv.Close()

	p := NewDeepSeekProvider(srv.Client())
	ch, err := p.Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "deepseek-chat", APIKey: "sk"}, ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
		Tools:    []ChatTool{{Name: "summon_card", Description: "d", Parameters: map[string]any{"type": "object"}}},
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
	if tool == nil || tool.Name != "summon_card" {
		t.Fatalf("tool use missing")
	}
	if tool.ArgsJSON != `{"card_id":"craap","reason":"r","nudge_text":"n"}` {
		t.Fatalf("reassembled args = %q", tool.ArgsJSON)
	}
	if usage == nil || usage.InputTokens != 120 || usage.OutputTokens != 45 {
		t.Fatalf("usage = %+v", usage)
	}
	if stop != StopToolCall {
		t.Fatalf("stop = %q", stop)
	}
}

// captureBody starts a server that records the request body it receives, then
// replies with a minimal valid SSE done frame. Returns the server and a pointer
// the test reads after Stream drains.
func captureBody(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
	}))
	return srv, &got
}

func drain(t *testing.T, p Provider, r Resolved) {
	t.Helper()
	ch, err := p.Stream(context.Background(), r, ChatRequest{Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch {
	}
}

func TestDeepSeekNeverDisablesThinking(t *testing.T) {
	// Regression guard: we do NOT send {"thinking":{"type":"disabled"}} for any
	// tier. Disabling it (2-3× faster) broke the coach's interdependent per-turn
	// tool decisions and no prompt design recovered them, so reasoning stays on
	// everywhere. See deepseek.go.
	srv, body := captureBody(t)
	defer srv.Close()
	p := NewDeepSeekProvider(srv.Client())
	for _, tier := range []string{"chaperone", "flagship", ""} {
		drain(t, p, Resolved{BaseURL: srv.URL, Model: "deepseek-v4-pro", APIKey: "sk", Tier: tier})
		if strings.Contains(*body, `"thinking"`) {
			t.Fatalf("tier %q must NOT set thinking, got: %s", tier, *body)
		}
	}
}

func TestDeepSeekSurfacesHTTPErrorWithoutLeaking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"secret-key-rejected"}}`)
	}))
	defer srv.Close()

	p := NewDeepSeekProvider(srv.Client())
	_, err := p.Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "m", APIKey: "sk"}, ChatRequest{})
	if err == nil {
		t.Fatal("want error on 401")
	}
	if strings.Contains(err.Error(), "secret-key-rejected") || strings.Contains(err.Error(), "sk") {
		t.Fatalf("error leaks upstream body/secret: %v", err)
	}
}
