package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// SSEWriter encodes Server-Sent Events with per-event flush. Streaming hygiene:
// text/event-stream content type, no-cache, X-Accel-Buffering: no so reverse
// proxies don't buffer. Each write flushes immediately.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter sets the SSE headers and returns a writer, or an error if the
// ResponseWriter cannot flush (no streaming support).
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("response writer does not support flushing")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	return &SSEWriter{w: w, flusher: fl}, nil
}

func (s *SSEWriter) writeEvent(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Text emits an assistant prose delta.
func (s *SSEWriter) Text(delta string) error {
	return s.writeEvent("text", map[string]string{"delta": delta})
}

// Card emits a summon_card proposal: the persisted card_instance id, the card id,
// the student-facing nudge, any AI-generated anchors (a JSON array; "[]" when
// none), and the id of the material this card is ABOUT. Carrying anchors here
// means the client has them at summon time rather than depending on a later
// refetch; carrying materialID means the client never has to guess which
// material a card targets from anchor contents or array position (whole-branch
// review finding [5] — a compare card's anchors span two materials by design,
// so guessing from them is exactly the coin flip this field exists to avoid).
func (s *SSEWriter) Card(cardInstanceID, cardID, nudgeText string, anchors []byte, materialID string) error {
	raw := json.RawMessage(anchors)
	if len(raw) == 0 {
		raw = json.RawMessage("[]")
	}
	return s.writeEvent("card", map[string]any{
		"card_instance_id": cardInstanceID,
		"card_id":          cardID,
		"nudge_text":       nudgeText,
		"anchors":          raw,
		"material_id":      materialID,
	})
}

// Phase announces a course phase transition (Slice 12): the structural floor
// was met and the coach's `advance` moved the student's session to a new
// phase. Reuses the same per-event write path as Text/Card — no generic
// Event method exists on this writer, so each frame kind gets its own method.
func (s *SSEWriter) Phase(to string) error {
	return s.writeEvent("phase", map[string]string{"to": to})
}

// Intervention emits one coach intervention (Slice 5c Studio turn).
func (s *SSEWriter) Intervention(interventionID, body, anchor, criterion, level string) error {
	return s.writeEvent("intervention", map[string]any{
		"intervention_id": interventionID,
		"body":            body,
		"anchor":          anchor,
		"criterion":       criterion,
		"level":           level,
	})
}

// Gate emits a gate-check result.
func (s *SSEWriter) Gate(contract, status string, passed, total int, missing []string) error {
	if missing == nil {
		missing = []string{}
	}
	return s.writeEvent("gate", map[string]any{
		"contract": contract, "status": status, "passed": passed, "total": total, "missing": missing,
	})
}

// Done ends the stream, carrying the persisted assistant message id.
func (s *SSEWriter) Done(messageID string) error {
	return s.writeEvent("done", map[string]string{"message_id": messageID})
}

// DoneCard ends a card-submit stream, additionally reporting the
// card_instance's RESULTING status ("active" when the submission did not
// satisfy the card's completion predicate and the row is still open for a
// refill/resubmit, "completed" when it did). The client uses THIS — never the
// bare Done — to decide whether to retire its local card state: nulling it
// unconditionally on every submit, regardless of outcome, destroyed a
// student's in-progress answers on every incomplete submit (FIX-D leaves an
// unsatisfied submit's card_instance "active" on purpose so she can refill it,
// but the client had no way to tell that apart from a genuine retire).
//
// It deliberately does NOT carry EvaluateCompletion's `missing` list. Nothing
// renders it, and a field on the wire that no reader consumes is the exact
// defect this slice rejected three times. Add it back with its renderer.
func (s *SSEWriter) DoneCard(cardStatus string) error {
	return s.writeEvent("done", map[string]any{"message_id": "", "card_status": cardStatus})
}

// ErrorEnvelope emits the standard error envelope as an SSE error event.
func (s *SSEWriter) ErrorEnvelope(code, message string) error {
	return s.writeEvent("error", map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// Review streams the whole-draft work-order as one event (the review is a
// batch, not incremental — spec §12, one snapshot = one review). items is
// the JSON-encoded []ReviewItem (or its wire equivalent).
func (s *SSEWriter) Review(items []byte) error {
	raw := json.RawMessage(items)
	if len(raw) == 0 {
		raw = json.RawMessage("[]")
	}
	return s.writeEvent("review", raw)
}

// Heartbeat writes an SSE comment to keep the connection alive.
func (s *SSEWriter) Heartbeat() error {
	if _, err := fmt.Fprint(s.w, ": ping\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
