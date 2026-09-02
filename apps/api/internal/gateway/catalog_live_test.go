package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"mindimprint/api/internal/config"
)

// Live checks against the real provider. Skipped unless LIVE_LLM=1 and a key is
// present, so the normal suite stays hermetic and offline.
//
// These exist because the failure this refactor guards against is INVISIBLE to
// a unit test: a thinking knob the upstream accepts and ignores produces a
// perfectly valid response — just a slow, expensive one. Only a live call with
// the reasoning-token count in hand can tell the difference.
//
//	LIVE_LLM=1 DASHSCOPE_API_KEY=... go test ./internal/gateway -run TestLive -v
func liveConfig(t *testing.T) config.Config {
	t.Helper()
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1 to run live provider checks")
	}
	key := os.Getenv("DASHSCOPE_API_KEY")
	if key == "" {
		t.Skip("DASHSCOPE_API_KEY not set")
	}
	// 🚨 The lane overrides have to come through, or this harness silently tests
	// the catalog default no matter what you asked for. It read only the key
	// once, so
	//
	//	LIVE_LLM=1 MODEL_CHAT=dashscope/kimi-k3 go test ./internal/gateway -run TestLive -v
	//
	// printed a reassuring PASS for deepseek-v4-pro and never called Kimi at all.
	// Comparing models is the entire reason this file exists, and every such
	// comparison would have been a measurement of the same model twice.
	// (Also pass -count=1: a cached PASS looks identical to a fresh one.)
	return config.Config{
		DashScopeKey:  key,
		ModelChat:     os.Getenv("MODEL_CHAT"),
		ModelFastChat: os.Getenv("MODEL_FAST_CHAT"),
		ModelEval:     os.Getenv("MODEL_EVAL"),
	}
}

func liveCollect(t *testing.T, r Resolved, req ChatRequest) (ChatResult, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	p := NewMuxProvider(map[string]Provider{
		KindOpenAICompatible: NewCatalogProvider(&http.Client{}),
		KindAnthropic:        NewAnthropicProvider(&http.Client{}),
	})
	start := time.Now()
	res, err := Collect(ctx, p, r, req)
	if err != nil {
		t.Fatalf("live call failed: %v", err)
	}
	return res, time.Since(start)
}

// The chaperone lane must come back with NO reasoning tokens. A non-zero count
// here means the thinking knob was accepted and ignored — the exact silent
// regression that motivated making the knob per-route data.
func TestLiveChaperoneLaneActuallyStopsReasoning(t *testing.T) {
	cfg := liveConfig(t)
	rs, err := NewResolvers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rs.Chat(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	res, elapsed := liveCollect(t, r, ChatRequest{
		MaxTokens: 200,
		Messages: []ChatMessage{
			{Role: RoleSystem, Content: "你是一位助教。"},
			{Role: RoleUser, Content: "一个班有23人，每人带2支笔，另有5支备用笔，总共多少支？只回答数字。"},
		},
	})
	t.Logf("chat lane %s: %v, in=%d out=%d text=%q",
		r.ModelID, elapsed.Round(time.Millisecond), res.Usage.InputTokens, res.Usage.OutputTokens, res.Text)

	if res.Text == "" {
		t.Error("no visible text came back")
	}
	if res.Usage.ReasoningTokens != nil && *res.Usage.ReasoningTokens > 0 {
		t.Errorf("chaperone lane still reasoned (%d reasoning tokens) — the thinking knob "+
			"for provider %q is being ignored upstream; fix thinkingOff in models.json",
			*res.Usage.ReasoningTokens, r.Provider)
	}
}

// Tool calling must survive the route, since the whole card/tool loop depends
// on it.
func TestLiveToolCallingWorksThroughTheCatalogRoute(t *testing.T) {
	cfg := liveConfig(t)
	rs, err := NewResolvers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rs.Eval(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := ChatRequest{
		MaxTokens: 400,
		Messages: []ChatMessage{
			{Role: RoleSystem, Content: "你是一位助教。需要查天气时调用工具。"},
			{Role: RoleUser, Content: "帮我查一下北京今天的天气。"},
		},
		Tools: []ChatTool{{
			Name:        "get_weather",
			Description: "查询某个城市的天气",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
				"required":   []string{"city"},
			},
		}},
	}

	// Read the stream directly: Collect keeps only a tool call's id and name,
	// and it is the reassembled ArgsJSON — streamed as fragments — that the
	// agent layer parses. A route that streams fragments the adapter cannot
	// rejoin would break the card loop while still looking like a tool call.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	start := time.Now()
	stream, err := NewCatalogProvider(&http.Client{}).Stream(ctx, r, req)
	if err != nil {
		t.Fatalf("live call failed: %v", err)
	}
	var calls []StreamToolUse
	for ev := range stream {
		if ev.Kind == EventToolUse && ev.ToolUse != nil {
			calls = append(calls, *ev.ToolUse)
		}
	}
	t.Logf("eval lane %s: %v, toolCalls=%d", r.ModelID, time.Since(start).Round(time.Millisecond), len(calls))

	if len(calls) == 0 {
		t.Fatal("no tool call — the card/tool loop would be broken on this route")
	}
	if calls[0].Name != "get_weather" {
		t.Errorf("tool name = %q", calls[0].Name)
	}
	if calls[0].ID == "" {
		t.Error("tool call carried no id — the tool-result message could not be addressed back")
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].ArgsJSON), &args); err != nil {
		t.Fatalf("streamed args did not reassemble into JSON (%q): %v", calls[0].ArgsJSON, err)
	}
	if args["city"] == nil {
		t.Errorf("tool args missing city: %#v", args)
	}
}
