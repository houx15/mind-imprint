package gateway

import (
	"context"
	"errors"
	"time"
)

// Collect drains a provider stream into a full ChatResult — text concatenated,
// last usage kept, stop reason recorded. Used by non-streaming callers
// (agent/coach.go's ProposeIntervention, agent/anchors.go's Generate, and
// agent/course.go's step render). Honors ctx cancellation via the underlying
// stream.
// 🚨 **不走流。** Collect 的语义是「我不要流，给我整份」，而 DashScope 那条聚合
// 口的流式回复会悄悄丢掉最后一块内容 —— 完整的理由与实测见 complete.go 的文件头。
// 对要解析 JSON 的调用，丢掉的那几个字符里必然有收尾的 `"}`，于是一次正确的回答
// 变成一句「JSON 没有写完」。
//
// 通道实现不了非流式（测试里的 stub、未来某个只给流的厂商）就照旧退回流式。
//
// 🚨 上游抖一下会**再试一次**（只有真的会自己好的那几类，见 retry.go）。
// 她正同步等着，所以是一次，不是三次；而流式那一路故意不在这里 ——
// 字节已经吐出去之后重试，等于拿另一份回复盖掉她屏幕上那半句话。
func Collect(ctx context.Context, p Provider, r Resolved, req ChatRequest) (ChatResult, error) {
	res, err := collectOnce(ctx, p, r, req)
	if err == nil || !Retryable(err) {
		return res, err
	}
	// 隔一下再来。上游 429 说的是「现在太挤」，立刻重放多半还是挤。
	// 等的时候也要认 ctx —— 她关掉页面之后不该还在这儿睡着。
	select {
	case <-ctx.Done():
		return ChatResult{}, err
	case <-time.After(retryPause):
	}
	res2, err2 := collectOnce(ctx, p, r, req)
	if err2 != nil {
		// 交出**第二次**的错。两次都坏的时候，后一次才是现在的实情。
		return ChatResult{}, err2
	}
	return res2, nil
}

// retryPause 两次之间等多久。
//
// 300ms：够让一阵瞬时拥塞过去，又不至于在她那边变成看得出来的卡顿
// （规划一轮本来就是 4–8 秒）。
const retryPause = 300 * time.Millisecond

func collectOnce(ctx context.Context, p Provider, r Resolved, req ChatRequest) (ChatResult, error) {
	if c, ok := p.(Completer); ok {
		res, err := c.Complete(ctx, r, req)
		if !errors.Is(err, errNotCompletable) {
			return res, err
		}
	}
	stream, err := p.Stream(ctx, r, req)
	if err != nil {
		return ChatResult{}, err
	}
	var res ChatResult
	for ev := range stream {
		switch ev.Kind {
		case EventTextDelta:
			res.Text += ev.TextDelta
		case EventReasoningDelta:
			// Kept apart from Text so no existing caller's reply body changes.
			// Callers that want to show the fold read Reasoning explicitly.
			res.Reasoning += ev.TextDelta
		case EventToolUse:
			if ev.ToolUse != nil {
				res.ToolCalls = append(res.ToolCalls, ToolCall{
					ID: ev.ToolUse.ID, Name: ev.ToolUse.Name, Args: decodeToolArgs(ev.ToolUse.ArgsJSON),
				})
			}
		case EventUsage:
			if ev.Usage != nil {
				res.Usage = *ev.Usage
			}
		case EventDone:
			res.StopReason = ev.StopReason
		}
	}
	return res, nil
}
