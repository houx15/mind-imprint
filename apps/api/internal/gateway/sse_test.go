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
	if err := s.Card("ci_1", "craap", "要不要核查一下来源？", []byte(`[{"id":"a0","question":"可信吗？"}]`), "mat_1"); err != nil {
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
	if !strings.Contains(body, `"card_id":"craap"`) || !strings.Contains(body, `"nudge_text":"要不要核查一下来源？"`) {
		t.Fatalf("card payload wrong:\n%s", body)
	}
	// Anchors ride the card event as a raw JSON array (not a re-escaped string).
	if !strings.Contains(body, `"anchors":[{`) || !strings.Contains(body, `"可信吗？"`) {
		t.Fatalf("card frame missing anchors:\n%s", body)
	}
	// The card's own material id rides the same frame — the client must never
	// have to guess it (whole-branch review finding [5]).
	if !strings.Contains(body, `"material_id":"mat_1"`) {
		t.Fatalf("card frame missing material_id:\n%s", body)
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

func TestSSEWriter_StudioEvents(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := NewSSEWriter(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Intervention("iid-1", "把它连到治理决心", "论证图 · 治理决心主张", "D5", "I2"); err != nil {
		t.Fatal(err)
	}
	if err := w.Gate("build_argument", "partial", 2, 7, []string{"concession 待完成"}); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"event: intervention", `"intervention_id":"iid-1"`, `"criterion":"D5"`, `"anchor":"论证图 · 治理决心主张"`,
		"event: gate", `"contract":"build_argument"`, `"passed":2`, `"total":7`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
