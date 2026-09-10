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

// complete_test.go —— Collect 必须拿到整份回答。
//
// 🚨 这里的假上游**照着实测的坏样子回**：流式那条路少送最后一块内容，然后照样
// 给一个 `finish_reason:"stop"`。这不是我编出来的形状，是 2026-09-10 用 curl 打
// DashScope 聚合口量到的（见 complete.go 的文件头）。
//
// 这个文件里的测试值得存在，是因为「Collect 走了哪条路」在代码里看不出对错：
// 两条路都返回 ChatResult，都不报错，差别只在末尾少几个字符 —— 而少的那几个
// 字符里有收尾的 `"}`。

// truncatingUpstream 是一个照着上游那个毛病回话的假服务器。
//
// stream=true 时少送最后一块；stream=false 时给完整的。
func truncatingUpstream(t *testing.T, full, dropped string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if s, _ := body["stream"].(bool); s {
			w.Header().Set("Content-Type", "text/event-stream")
			// 内容分片：**少送最后一块**。
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"content":`+quote(full)+`},"finish_reason":null}]}`+"\n\n")
			// 然后照样宣称正常收尾，并报一个包含了那一块的 token 数。
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":55}}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"content":`+quote(full+dropped)+
			`},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":55}}`)
	}))
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func testResolved(base string) Resolved {
	return Resolved{
		Provider: "dashscope", Kind: KindOpenAICompatible,
		Model: "deepseek-v4-flash", ModelID: "dashscope/deepseek-v4-flash",
		BaseURL: base, APIKey: "test-key",
	}
}

// 🚨 这一条是那次事故的回归测试。
func TestCollectGetsTheWholeReplyEvenWhenTheStreamWouldDropItsTail(t *testing.T) {
	const head = `{"titleZh":"标题","hook":"站得住吗？","evidence":"prolonging global`
	const tail = ` warming."}`

	srv := truncatingUpstream(t, head, tail)
	defer srv.Close()

	p := NewMuxProvider(map[string]Provider{
		KindOpenAICompatible: NewCatalogProvider(srv.Client()),
	})
	res, err := Collect(context.Background(), p, testResolved(srv.URL), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if res.Text != head+tail {
		t.Fatalf("Collect 拿到的是被截断的那份：\n got %q\nwant %q", res.Text, head+tail)
	}
	// 少了收尾的 `"}` 就不是合法 JSON —— 这正是线上那一整天没有星图的样子。
	var into map[string]any
	if err := json.Unmarshal([]byte(res.Text), &into); err != nil {
		t.Fatalf("拿到的东西不是完整的 JSON：%v", err)
	}
	if res.Usage.OutputTokens != 55 {
		t.Errorf("用量没带出来：%+v", res.Usage)
	}
	if res.StopReason != StopStop {
		t.Errorf("stop reason = %q", res.StopReason)
	}
}

// 上游只肯给流的时候（或者测试里那些只实现了 Stream 的 stub），Collect 照旧能用。
func TestCollectFallsBackToStreamingWhenTheProviderCannotComplete(t *testing.T) {
	p := streamOnlyProvider{text: "hello"}
	res, err := Collect(context.Background(), p, Resolved{}, ChatRequest{})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if res.Text != "hello" {
		t.Errorf("退回流式之后拿到 %q", res.Text)
	}
}

type streamOnlyProvider struct{ text string }

func (s streamOnlyProvider) Stream(context.Context, Resolved, ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Kind: EventTextDelta, TextDelta: s.text}
	ch <- StreamEvent{Kind: EventDone, StopReason: StopStop}
	close(ch)
	return ch, nil
}

// mux 底下挂着一个只会流的通道时，Complete 要说「我做不到」，而不是报一个真错误
// —— 报错会让 Collect 直接失败，而它本该退回流式。
func TestMuxCompleteSaysItCannotRatherThanFailing(t *testing.T) {
	m := NewMuxProvider(map[string]Provider{
		KindOpenAICompatible: streamOnlyProvider{text: "x"},
	})
	_, err := m.Complete(context.Background(), Resolved{Kind: KindOpenAICompatible}, ChatRequest{})
	if err != errNotCompletable {
		t.Fatalf("err = %v，want errNotCompletable", err)
	}
}

// 非流式请求必须真的把 stream 关掉。漏掉的话上游按 SSE 回、这边按 JSON 读，
// 症状是「解析不出任何东西」，看上去又像模型的错。
func TestCompleteTurnsStreamOff(t *testing.T) {
	var gotStream any = "unset"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		gotStream = body["stream"]
		if !strings.Contains(r.Header.Get("Accept"), "application/json") {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	p := NewCatalogProvider(srv.Client())
	if _, err := p.Complete(context.Background(), testResolved(srv.URL), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if b, ok := gotStream.(bool); !ok || b {
		t.Fatalf("请求里的 stream = %v，必须是 false", gotStream)
	}
}
