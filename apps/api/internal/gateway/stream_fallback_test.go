package gateway

import (
	"context"
	"testing"
)

// stream_fallback_test.go —— 一条「流式会丢尾巴」的通道不许走流。
//
// 假上游用的是 complete_test.go 里那个 truncatingUpstream：它照实测的坏样子回
// （stream=true 少送最后一块，stream=false 完整）。判据不是「调用成功了」，是
// **拿到的那份到底全不全**。

// 🚨 带着标记的通道：调用 Stream，拿到的必须是完整的那份。
func TestStreamIsNotUsedOnARouteThatDropsItsTail(t *testing.T) {
	const head = "印记说了一句话，最后两个字是"
	const tail = "结尾。"

	srv := truncatingUpstream(t, head, tail)
	defer srv.Close()

	r := testResolved(srv.URL)
	r.Policy.StreamDropsTail = true

	p := NewMuxProvider(map[string]Provider{
		KindOpenAICompatible: NewCatalogProvider(srv.Client()),
	})
	ch, err := p.Stream(context.Background(), r, ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var got string
	var usage ChatUsage
	var stop StopReason
	for ev := range ch {
		switch ev.Kind {
		case EventTextDelta:
			got += ev.TextDelta
		case EventUsage:
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case EventDone:
			stop = ev.StopReason
		}
	}
	if got != head+tail {
		t.Fatalf("Stream 仍然拿到了被截断的那份：\n got %q\nwant %q", got, head+tail)
	}
	// 事件的形状要和真流式一样，否则调用方得为这一条通道写特例。
	if usage.OutputTokens != 55 {
		t.Errorf("用量没跟着出来：%+v", usage)
	}
	if stop != StopStop {
		t.Errorf("stop reason = %q", stop)
	}
}

// 没有那个标记的通道照旧走真流式 —— 这是给一个坏掉的模型开的口子，不是给所有人
// 换实现。
func TestStreamStillStreamsOnANormalRoute(t *testing.T) {
	const head = "正常通道的回答"
	srv := truncatingUpstream(t, head, "（这段只有非流式才拿得到）")
	defer srv.Close()

	p := NewCatalogProvider(srv.Client())
	ch, err := p.Stream(context.Background(), testResolved(srv.URL), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var got string
	for ev := range ch {
		if ev.Kind == EventTextDelta {
			got += ev.TextDelta
		}
	}
	if got != head {
		t.Fatalf("普通通道没有走真流式：got %q want %q", got, head)
	}
}

// 目录里那条真的模型确实带着这个标记 —— 少了它，上面两条测试保护的是一个没人
// 用的字段。
func TestTheMeasuredBrokenModelCarriesTheFlag(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	m, ok := c.Models["dashscope/deepseek-v4-flash"]
	if !ok {
		t.Skip("目录里已经没有这个模型了")
	}
	if !m.StreamDropsTail {
		t.Error("dashscope/deepseek-v4-flash 没有标 streamDropsTail —— " +
			"2026-09-11 实测它的流式回复 0/3 到齐，非流式 3/3")
	}
}
