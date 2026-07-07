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
// the student-facing nudge, and any AI-generated anchors (a JSON array; "[]" when
// none). Carrying anchors here means the client has them at summon time rather
// than depending on a later refetch.
func (s *SSEWriter) Card(cardInstanceID, cardID, nudgeText string, anchors []byte) error {
	raw := json.RawMessage(anchors)
	if len(raw) == 0 {
		raw = json.RawMessage("[]")
	}
	return s.writeEvent("card", map[string]any{
		"card_instance_id": cardInstanceID,
		"card_id":          cardID,
		"nudge_text":       nudgeText,
		"anchors":          raw,
	})
}

// Done ends the stream, carrying the persisted assistant message id.
func (s *SSEWriter) Done(messageID string) error {
	return s.writeEvent("done", map[string]string{"message_id": messageID})
}

// ErrorEnvelope emits the standard error envelope as an SSE error event.
func (s *SSEWriter) ErrorEnvelope(code, message string) error {
	return s.writeEvent("error", map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// Heartbeat writes an SSE comment to keep the connection alive.
func (s *SSEWriter) Heartbeat() error {
	if _, err := fmt.Fprint(s.w, ": ping\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
