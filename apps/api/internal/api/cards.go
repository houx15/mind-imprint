package api

import (
	"encoding/json"

	"mindimprint/api/internal/httpx"
)

// The old task-scoped card handlers (patchCard/putCard/skipCard, PATCH|PUT
// /api/v1/tasks/{id}/cards/{cid}, POST .../skip) were retired in Slice 5d —
// card mutation now goes through the project-scoped routes in
// projectcards.go. These envelope validators survive because
// projectcards.go's activate/skip/submit handlers still call them.

// traceKinds is the closed set of TraceEvent kinds (unchanged contract).
var traceKinds = map[string]bool{
	"field_change": true, "step_expand": true, "note_open": true, "skip": true, "submit": true,
}

// validateFieldValues requires a JSON object.
func validateFieldValues(raw json.RawMessage) error {
	var obj map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return httpx.ErrBadRequest("validation_failed", "field_values 必须是对象", nil)
	}
	return nil
}

// validateEventTrace requires a JSON array whose every element has a known kind.
func validateEventTrace(raw json.RawMessage) error {
	var arr []map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &arr) != nil {
		return httpx.ErrBadRequest("validation_failed", "event_trace 必须是对象数组", nil)
	}
	for _, ev := range arr {
		kind, _ := ev["kind"].(string)
		if !traceKinds[kind] {
			return httpx.ErrBadRequest("validation_failed", "event_trace 含未知事件类型", nil)
		}
	}
	return nil
}

// validateAnchors requires a JSON array whose every element is a well-formed
// anchor (structural check only — existence of material/block is not verified).
func validateAnchors(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil // absent anchors is allowed (defaults to [])
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil {
		return httpx.ErrBadRequest("validation_failed", "anchors 必须是数组", nil)
	}
	for _, a := range arr {
		author, _ := a["author"].(string)
		if author != "ai" && author != "student" {
			return httpx.ErrBadRequest("validation_failed", "anchors.author 非法", nil)
		}
		for _, k := range []string{"block_id", "dimension", "question", "quote", "answer", "material_id", "id"} {
			if _, ok := a[k].(string); !ok {
				return httpx.ErrBadRequest("validation_failed", "anchors 字段缺失或类型错误", nil)
			}
		}
		if _, ok := a["start"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.start 必须是数字", nil)
		}
		if _, ok := a["end"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.end 必须是数字", nil)
		}
	}
	return nil
}
