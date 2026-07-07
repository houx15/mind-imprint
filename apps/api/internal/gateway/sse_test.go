package gateway

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEWriterEncodesEvents(t *testing.T) {
	rec := httptest.NewRecorder()
	s, err := NewSSEWriter(rec)
	if err != nil {
		t.Fatalf("NewSSEWriter: %v", err)
	}
	if err := s.Text("你好"); err != nil {
		t.Fatalf("Text: %v", err)
	}
	if err := s.Card("ci_1", "sift_craap", "要不要核查一下来源？", []byte(`[{"id":"a0","question":"可信吗？"}]`)); err != nil {
		t.Fatalf("Card: %v", err)
	}
	if err := s.Done("msg_1"); err != nil {
		t.Fatalf("Done: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: text\ndata: {\"delta\":\"你好\"}\n\n") {
		t.Fatalf("text frame wrong:\n%s", body)
	}
	if !strings.Contains(body, "event: card\ndata: ") || !strings.Contains(body, `"card_instance_id":"ci_1"`) {
		t.Fatalf("card frame wrong:\n%s", body)
	}
	if !strings.Contains(body, `"card_id":"sift_craap"`) || !strings.Contains(body, `"nudge_text":"要不要核查一下来源？"`) {
		t.Fatalf("card payload wrong:\n%s", body)
	}
	// Anchors ride the card event as a raw JSON array (not a re-escaped string).
	if !strings.Contains(body, `"anchors":[{`) || !strings.Contains(body, `"可信吗？"`) {
		t.Fatalf("card frame missing anchors:\n%s", body)
	}
	if !strings.Contains(body, "event: done\ndata: {\"message_id\":\"msg_1\"}\n\n") {
		t.Fatalf("done frame wrong:\n%s", body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("missing X-Accel-Buffering: no")
	}
}
