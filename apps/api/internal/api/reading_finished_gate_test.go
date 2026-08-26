package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// finishReadingAtom writes a takeaway (finish's own gate) and finishes the
// reading, failing the test if either step doesn't return 200.
func finishReadingAtom(t *testing.T, h http.Handler, cookie *http.Cookie, id string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"我的收获。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT takeaway (setup) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/finish",
		strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST finish (setup) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestFinishedReading_RejectsMutatingRequests — Task 17: every mutating lite
// reading route must be refused, 403 reading_finished, once the reading is
// finished. 铁律④: the process record is evidence a report is generated
// from, so it must not be able to keep changing after the student (or a
// stray tab, or a raw API call) considers it done.
func TestFinishedReading_RejectsMutatingRequests(t *testing.T) {
	cardID := "00000000-0000-0000-0000-0000000000cd"

	cases := []struct {
		name   string
		method string
		path   func(id string) string
		body   string
	}{
		{"rename", "PATCH", func(id string) string { return "/api/v1/readings/" + id }, `{"title":"新标题"}`},
		{"put_source", "PUT", func(id string) string { return "/api/v1/readings/" + id + "/source" }, `{"kind":"paste","text":"正文"}`},
		{"post_turn", "POST", func(id string) string { return "/api/v1/readings/" + id + "/turn" }, `{"studentText":"hi"}`},
		{"put_brief", "PUT", func(id string) string { return "/api/v1/readings/" + id + "/brief" }, `{}`},
		{"put_takeaway", "PUT", func(id string) string { return "/api/v1/readings/" + id + "/takeaway" }, `{"text":"新的收获。"}`},
		{"post_annotations", "POST", func(id string) string { return "/api/v1/readings/" + id + "/annotations" }, `{"blockId":"b1","span":[0,1],"quote":"x","note":"n"}`},
		{"activate_card", "POST", func(id string) string { return "/api/v1/readings/" + id + "/cards/" + cardID + "/activate" }, `{}`},
		{"skip_card", "POST", func(id string) string { return "/api/v1/readings/" + id + "/cards/" + cardID + "/skip" }, `{}`},
		{"submit_card", "POST", func(id string) string { return "/api/v1/readings/" + id + "/cards/" + cardID + "/submit" }, `{}`},
		{"evaluate_card", "POST", func(id string) string { return "/api/v1/readings/" + id + "/cards/" + cardID + "/evaluate" }, `{}`},
		{"summon", "POST", func(id string) string { return "/api/v1/readings/" + id + "/summon" }, `{"cardId":"craap"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie, _, _ := liteHandler(t)
			id := createReadingAtom(t, h, cookie)
			finishReadingAtom(t, h, cookie, id)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest(tc.method, tc.path(id), strings.NewReader(tc.body)), cookie))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s on a finished reading = %d, want 403; body=%s", tc.method, tc.path(id), rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), `"code":"reading_finished"`) {
				t.Fatalf("%s %s body missing reading_finished code — got %s", tc.method, tc.path(id), rec.Body)
			}
		})
	}
}

// TestFinishedReading_ReadsStillWork — she must still be able to open a
// finished reading and see her takeaway, transcript, and cards. The gate is
// method-based (non-GET only), so every read stays exactly as before.
func TestFinishedReading_ReadsStillWork(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	// Give it a source before finishing, so GET /source has something to read.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source",
		strings.NewReader(`{"kind":"paste","text":"这是正文，足够长的一段测试文字。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT source (setup) = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	finishReadingAtom(t, h, cookie, id)

	reads := []struct {
		name string
		path string
	}{
		{"get_reading", "/api/v1/readings/" + id},
		{"get_source", "/api/v1/readings/" + id + "/source"},
		{"get_messages", "/api/v1/readings/" + id + "/messages"},
		{"get_takeaway", "/api/v1/readings/" + id + "/takeaway"},
		{"get_brief", "/api/v1/readings/" + id + "/brief"},
		{"get_cards", "/api/v1/readings/" + id + "/cards"},
		{"get_annotations", "/api/v1/readings/" + id + "/annotations"},
	}
	for _, rd := range reads {
		t.Run(rd.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", rd.path, nil), cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s on a finished reading = %d, want 200; body=%s", rd.path, rec.Code, rec.Body)
			}
		})
	}

	// The finished reading's own DTO reports its status, for good measure.
	var out struct {
		Status string `json:"status"`
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id, nil), cookie))
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Status != "finished" {
		t.Fatalf("status = %q, want finished (err=%v)", out.Status, err)
	}
}

// TestActiveReading_UnaffectedByGate — the regression risk that matters
// most: every existing lite test drives ACTIVE readings, so the gate must
// be a strict no-op for them. A representative mutating call on a
// brand-new (active) reading must succeed exactly as before.
func TestActiveReading_UnaffectedByGate(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"我的收获。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT takeaway on an active reading = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/readings/"+id,
		strings.NewReader(`{"title":"新标题"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH rename on an active reading = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}
