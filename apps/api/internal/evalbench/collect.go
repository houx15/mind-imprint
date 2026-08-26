package evalbench

import (
	"context"
	"errors"

	"mindimprint/api/internal/gateway"
)

var errIncompleteStream = errors.New("evalbench: provider stream incomplete")

// collectEvalbench is intentionally stricter than gateway.Collect. Its policy
// applies only to experiment attempts: ordinary production callers retain the
// gateway's historical best-effort terminal semantics.
func collectEvalbench(ctx context.Context, p gateway.Provider, r gateway.Resolved, req gateway.ChatRequest) (gateway.ChatResult, error) {
	stream, err := p.Stream(ctx, r, req)
	if err != nil {
		return gateway.ChatResult{}, err
	}
	var result gateway.ChatResult
	seenDone := false
	incomplete := false
	for event := range stream {
		switch event.Kind {
		case gateway.EventTextDelta:
			result.Text += event.TextDelta
		case gateway.EventToolUse:
			if event.ToolUse != nil {
				result.ToolCalls = append(result.ToolCalls, gateway.ToolCall{ID: event.ToolUse.ID, Name: event.ToolUse.Name})
			}
		case gateway.EventUsage:
			if event.Usage != nil {
				result.Usage = *event.Usage
			}
		case gateway.EventDone:
			seenDone = true
			result.StopReason = event.StopReason
			if event.Incomplete {
				incomplete = true
			}
		}
	}
	if ctx.Err() != nil {
		return gateway.ChatResult{}, ctx.Err()
	}
	if !seenDone {
		return gateway.ChatResult{}, errIncompleteStream
	}
	if incomplete {
		return gateway.ChatResult{}, errIncompleteStream
	}
	return result, nil
}
