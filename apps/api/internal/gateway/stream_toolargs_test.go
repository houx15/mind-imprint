package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 🚨 工具调用必须**带着参数**过来。
//
// 这条是写 streamViaComplete 时自己撞出来的：第一版从 ChatResult 取工具调用，
// 而 ChatResult 里的 ToolCall 不带参数（Collect 从流式收上来的也不带）。于是这条
// 通道发出去的 EventToolUse 全是空参数的 —— 调用方多半把它当成一次空调用跳过，
// 屏幕上的表现是「印记说要给你一张卡，然后什么也没发生」。
func TestStreamViaCompleteCarriesToolArguments(t *testing.T) {
	srv := toolCallUpstream(t)
	defer srv.Close()

	r := testResolved(srv.URL)
	r.Policy.StreamDropsTail = true

	p := NewCatalogProvider(srv.Client())
	ch, err := p.Stream(context.Background(), r, ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
		Tools:    []ChatTool{{Name: "summon_card"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var uses []StreamToolUse
	for ev := range ch {
		if ev.Kind == EventToolUse && ev.ToolUse != nil {
			uses = append(uses, *ev.ToolUse)
		}
	}
	if len(uses) != 1 {
		t.Fatalf("拿到 %d 个工具调用，want 1", len(uses))
	}
	if uses[0].Name != "summon_card" {
		t.Errorf("工具名 = %q", uses[0].Name)
	}
	if uses[0].ArgsJSON == "" {
		t.Fatal("工具调用没有带参数 —— 调用方会把它当成一次空调用")
	}
	if !strings.Contains(uses[0].ArgsJSON, "craap") {
		t.Errorf("参数不对：%q", uses[0].ArgsJSON)
	}
}

// toolCallUpstream 是一个只回一次工具调用的假上游（非流式形状）。
func toolCallUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"content":"",`+
			`"tool_calls":[{"id":"call_1","type":"function","function":{`+
			`"name":"summon_card","arguments":"{\"card_id\":\"craap\"}"}}]},`+
			`"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":9}}`)
	}))
}
