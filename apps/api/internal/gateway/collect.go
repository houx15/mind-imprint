package gateway

import (
	"context"
	"errors"
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
func Collect(ctx context.Context, p Provider, r Resolved, req ChatRequest) (ChatResult, error) {
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
