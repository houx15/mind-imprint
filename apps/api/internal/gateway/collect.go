package gateway

import "context"

// Collect drains a provider stream into a full ChatResult — text concatenated,
// last usage kept, stop reason recorded. Used by non-streaming callers
// (agent/coach.go's ProposeIntervention, agent/anchors.go's Generate, and
// agent/course.go's step render). Honors ctx cancellation via the underlying
// stream.
func Collect(ctx context.Context, p Provider, r Resolved, req ChatRequest) (ChatResult, error) {
	stream, err := p.Stream(ctx, r, req)
	if err != nil {
		return ChatResult{}, err
	}
	var res ChatResult
	for ev := range stream {
		switch ev.Kind {
		case EventTextDelta:
			res.Text += ev.TextDelta
		case EventToolUse:
			if ev.ToolUse != nil {
				res.ToolCalls = append(res.ToolCalls, ToolCall{ID: ev.ToolUse.ID, Name: ev.ToolUse.Name})
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
