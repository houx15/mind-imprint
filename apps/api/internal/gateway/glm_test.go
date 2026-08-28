package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGLMStreamsOpenAICompatibleEventsAndBuildsSupportedRequest(t *testing.T) {
	var requestBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer zai-secret" {
			t.Errorf("authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"reasoning_content":"private reasoning"}}]}`,
			`data: {"choices":[{"delta":{"content":"可见回复"}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"summon_card","arguments":"{\"card_id\":\"cr"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"aap\"}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":12,"completion_tokens":20,"completion_tokens_details":{"reasoning_tokens":15}}}`,
			`data: [DONE]`,
			``,
		}, "\n\n"))
	}))
	defer srv.Close()

	p := NewGLMProvider(srv.Client())
	ch, err := p.Stream(context.Background(), Resolved{BaseURL: srv.URL + "/", Model: "glm-5.3-flash", APIKey: "zai-secret"}, ChatRequest{
		MaxTokens:      2468,
		ResponseFormat: ResponseFormatJSONObject,
		Messages:       []ChatMessage{{Role: RoleUser, Content: "hi"}},
		Tools:          []ChatTool{{Name: "summon_card", Description: "d", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	var tool *StreamToolUse
	var usage *ChatUsage
	for ev := range ch {
		switch ev.Kind {
		case EventTextDelta:
			text.WriteString(ev.TextDelta)
		case EventToolUse:
			tool = ev.ToolUse
		case EventUsage:
			usage = ev.Usage
		}
	}
	if text.String() != "可见回复" {
		t.Fatalf("visible text = %q", text.String())
	}
	if tool == nil || tool.ID != "call_1" || tool.Name != "summon_card" || tool.ArgsJSON != `{"card_id":"craap"}` {
		t.Fatalf("tool = %#v", tool)
	}
	if usage == nil || usage.InputTokens != 12 || usage.OutputTokens != 20 || usage.ReasoningTokens == nil || *usage.ReasoningTokens != 15 {
		t.Fatalf("usage = %#v", usage)
	}

	if requestBody["model"] != "glm-5.3-flash" || requestBody["max_tokens"] != float64(2468) || requestBody["stream"] != true || requestBody["tool_stream"] != true {
		t.Fatalf("unexpected request body: %#v", requestBody)
	}
	if requestBody["temperature"] != float64(1) || requestBody["top_p"] != 0.95 || requestBody["reasoning_effort"] != "max" {
		t.Fatalf("GLM defaults missing: %#v", requestBody)
	}
	thinking, _ := requestBody["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
		t.Fatalf("thinking = %#v", thinking)
	}
	format, _ := requestBody["response_format"].(map[string]any)
	if format["type"] != "json_object" {
		t.Fatalf("response format = %#v", format)
	}
}

func TestGLMRejectsDisableThinkingBeforeRequest(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer srv.Close()

	_, err := NewGLMProvider(srv.Client()).Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "glm-5.3-flash", APIKey: "key"}, ChatRequest{DisableThinking: true})
	if err == nil || !strings.Contains(err.Error(), "does not support disabling thinking") {
		t.Fatalf("error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("requests = %d, want 0", calls)
	}
}

func TestGLMResolvedReasoningEffortAndRequestOverride(t *testing.T) {
	p := NewGLMProvider(nil)
	resolved := Resolved{Model: "glm-5.3-flash", DefaultReasoningEffort: "low"}
	low, err := p.buildBody(resolved, ChatRequest{})
	if err != nil || low["reasoning_effort"] != "low" {
		t.Fatalf("profile effort = %#v, err = %v", low["reasoning_effort"], err)
	}
	high, err := p.buildBody(resolved, ChatRequest{ReasoningEffort: "high"})
	if err != nil || high["reasoning_effort"] != "high" {
		t.Fatalf("request override = %#v, err = %v", high["reasoning_effort"], err)
	}
}

func TestGLMSurfacesHTTPErrorWithoutLeaking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"zai-secret-rejected"}}`)
	}))
	defer srv.Close()

	_, err := NewGLMProvider(srv.Client()).Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "glm-5.3-flash", APIKey: "zai-secret"}, ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "glm http 401") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "zai-secret") || strings.Contains(err.Error(), "zai-secret-rejected") {
		t.Fatalf("error leaks secret/upstream body: %v", err)
	}
}

func TestGLMMarksMissingDoneAsIncomplete(t *testing.T) {
	srv := cannedSSE(t, `data: {"choices":[{"delta":{"content":"partial"}}]}`)
	defer srv.Close()

	ch, err := NewGLMProvider(srv.Client()).Stream(context.Background(), Resolved{BaseURL: srv.URL, Model: "glm-5.3-flash", APIKey: "key"}, ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	incomplete := false
	for ev := range ch {
		if ev.Kind == EventDone {
			incomplete = ev.Incomplete
		}
	}
	if !incomplete {
		t.Fatal("missing [DONE] was not marked incomplete")
	}
}
