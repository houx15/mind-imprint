package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// postWritingStage is a small helper: POST /api/v1/writings/{id}/stage with
// {"stage": stage}, returning the recorded response.
func postWritingStage(t *testing.T, h http.Handler, cookie *http.Cookie, id, stage string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"stage":"` + stage + `"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/stage", body), cookie))
	return rec
}

// TestSetWritingStage_SkippingAheadIs200 — the four stages are a map, not a
// gate (铁律②): jumping straight from ideate to snippets, skipping outline
// entirely, must succeed.
func TestSetWritingStage_SkippingAheadIs200(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "跳过大纲直接写片段")

	rec := postWritingStage(t, h, cookie, id, "snippets")
	if rec.Code != http.StatusOK {
		t.Fatalf("skip ideate->snippets = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Stage != "snippets" {
		t.Fatalf("stage = %q, want snippets (err=%v)", out.Stage, err)
	}
}

// TestSetWritingStage_SkipIsRecorded — a skip is allowed AND recorded
// (铁律④): the stage change writes one atom_message role='system' whose
// content records from→to, even though (especially though) it skipped a
// stage.
func TestSetWritingStage_SkipIsRecorded(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "跳过大纲的追踪")

	rec := postWritingStage(t, h, cookie, id, "snippets")
	if rec.Code != http.StatusOK {
		t.Fatalf("stage change = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	msgs, err := q.ListAtomMessages(t.Context(), mustUUID(id))
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	// seq=1 is the opening idea (role='student'), written by createWriting.
	if len(msgs) != 2 {
		t.Fatalf("got %d atom_messages, want 2 (opening idea + stage trace)", len(msgs))
	}
	trace := msgs[1]
	if trace.Role != "system" {
		t.Fatalf("trace role = %q, want system", trace.Role)
	}
	if !strings.Contains(trace.Content, "ideate") || !strings.Contains(trace.Content, "snippets") {
		t.Fatalf("trace content = %q, want it to record ideate->snippets", trace.Content)
	}
}

// TestSetWritingStage_BackwardIsAllowed — going backward (snippets->outline)
// is normal, not an error: it's normal for a student to want to go back and
// fill in the outline.
func TestSetWritingStage_BackwardIsAllowed(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "先写片段再回头补大纲")

	if rec := postWritingStage(t, h, cookie, id, "snippets"); rec.Code != http.StatusOK {
		t.Fatalf("ideate->snippets = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	rec := postWritingStage(t, h, cookie, id, "outline")
	if rec.Code != http.StatusOK {
		t.Fatalf("snippets->outline (backward) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Stage != "outline" {
		t.Fatalf("stage = %q, want outline (err=%v)", out.Stage, err)
	}

	msgs, err := q.ListAtomMessages(t.Context(), mustUUID(id))
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	// seq=1 opening idea, seq=2 ideate->snippets trace, seq=3 snippets->outline trace.
	if len(msgs) != 3 {
		t.Fatalf("got %d atom_messages, want 3", len(msgs))
	}
	if !strings.Contains(msgs[2].Content, "snippets") || !strings.Contains(msgs[2].Content, "outline") {
		t.Fatalf("backward trace content = %q, want it to record snippets->outline", msgs[2].Content)
	}
}

// TestSetWritingStage_InvalidValueIs400 — an invalid stage value must be a
// clean 400 (validated in Go), never a 500 surfaced from the CHECK
// constraint.
func TestSetWritingStage_InvalidValueIs400(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "非法阶段值")

	rec := postWritingStage(t, h, cookie, id, "publishing")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid stage = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "invalid_stage") {
		t.Fatalf("want invalid_stage code, got %s", rec.Body)
	}
}

// TestSetWritingTargetWords_ValidAtAnyStage — targetWords is never a
// precondition for anything (2026-08-27 product ruling): it can be set at
// any stage, and setting it does not touch stage.
func TestSetWritingTargetWords_ValidAtAnyStage(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "任意阶段设置目标字数")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/target-words",
		strings.NewReader(`{"targetWords":800}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT target-words = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Stage       string `json:"stage"`
		TargetWords *int32 `json:"targetWords"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.TargetWords == nil || *out.TargetWords != 800 {
		t.Fatalf("targetWords = %v, want 800", out.TargetWords)
	}
	if out.Stage != "ideate" {
		t.Fatalf("stage = %q, want ideate unchanged — target-words must not touch stage", out.Stage)
	}
}

// TestSetWritingTargetWords_OutOfRangeIs400 — must be a positive integer with
// an upper bound; 0 and > 100000 are both refused.
func TestSetWritingTargetWords_OutOfRangeIs400(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "越界的目标字数")

	for _, body := range []string{`{"targetWords":0}`, `{"targetWords":100001}`, `{"targetWords":-5}`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/target-words",
			strings.NewReader(body)), cookie))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s => %d, want 400; resp=%s", body, rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "invalid_target_words") {
			t.Fatalf("want invalid_target_words code, got %s", rec.Body)
		}
	}
}

// TestWritingReachesDraft_TargetWordsStillNull pins the 2026-08-27 product
// ruling: a writing can reach the 'draft' stage with target_words still
// NULL. This must never become a gate — a later task must not reintroduce
// one.
func TestWritingReachesDraft_TargetWordsStillNull(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "从不设置目标字数也能到成稿")

	for _, stage := range []string{"outline", "snippets", "draft"} {
		rec := postWritingStage(t, h, cookie, id, stage)
		if rec.Code != http.StatusOK {
			t.Fatalf("stage -> %s = %d, want 200; body=%s", stage, rec.Code, rec.Body)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Stage       string `json:"stage"`
		TargetWords *int32 `json:"targetWords"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Stage != "draft" {
		t.Fatalf("stage = %q, want draft", out.Stage)
	}
	if out.TargetWords != nil {
		t.Fatalf("targetWords = %v, want still nil — length must never be a precondition", out.TargetWords)
	}
}
