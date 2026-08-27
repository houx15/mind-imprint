package api_test

// writing_setup_test.go — the entry dialog and the coach's opening line.
//
// Both exist because of what a real walk of production found on 2026-08-27:
// the room was silent when you opened it, and setting a word count appeared
// to do nothing. These tests pin the fixes so neither can quietly return.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func putWritingSetup(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/setup", strings.NewReader(body)), cookie))
	return rec
}

type writingSetupDTO struct {
	Lang        string  `json:"lang"`
	TargetWords *int32  `json:"targetWords"`
	SetupAt     *string `json:"setupAt"`
	Stage       string  `json:"stage"`
}

// TestWritingSetup_RecordsLangLengthAndStamp — the whole point of the dialog:
// after it, the room knows the language and the length, and knows not to ask
// again.
func TestWritingSetup_RecordsLangLengthAndStamp(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "I want to write about phones at school.")

	rec := putWritingSetup(t, h, cookie, id, `{"lang":"en","targetWords":800,"note":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out writingSetupDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode setup: %v — body=%s", err, rec.Body)
	}
	if out.Lang != "en" {
		t.Fatalf("lang = %q, want en — a freely-typed English idea used to be forced to zh", out.Lang)
	}
	if out.TargetWords == nil || *out.TargetWords != 800 {
		t.Fatalf("targetWords = %v, want 800", out.TargetWords)
	}
	if out.SetupAt == nil || *out.SetupAt == "" {
		t.Fatalf("setupAt not stamped — the dialog would reopen on every visit")
	}
}

// TestWritingSetup_LengthStaysOptional — 铁律②: length is never a
// precondition. Omitting it must succeed and leave it null, not default to a
// number nobody chose.
func TestWritingSetup_LengthStaysOptional(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")

	rec := putWritingSetup(t, h, cookie, id, `{"lang":"zh"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup without length = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out writingSetupDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.TargetWords != nil {
		t.Fatalf("targetWords = %v, want nil — she chose not to set one", out.TargetWords)
	}
	if out.SetupAt == nil {
		t.Fatalf("setup without a length must still count as done")
	}
}

// TestWritingSetup_NoteBecomesHerOwnTurn — the free-text box replaced a 文体
// radio precisely so she could answer in her own words. Those words have to
// reach the transcript, or every downstream prompt is blind to them.
func TestWritingSetup_NoteBecomesHerOwnTurn(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "手机该不该禁。")

	const note = "我们班上学期真的收过手机，我想写这件事对我的影响。"
	if rec := putWritingSetup(t, h, cookie, id, `{"lang":"zh","targetWords":600,"note":"`+note+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	atomID, perr := uuid.Parse(id)
	if perr != nil {
		t.Fatalf("parse writing id: %v", perr)
	}
	msgs, err := q.ListAtomMessages(context.Background(), atomID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	found := false
	for _, m := range msgs {
		if m.Role == "student" && strings.Contains(m.Content, "上学期真的收过手机") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the note never became a student turn; messages=%+v", msgs)
	}
}

// TestWritingSetup_RejectsUnknownLang — the only two the room can render.
func TestWritingSetup_RejectsUnknownLang(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "x")
	if rec := putWritingSetup(t, h, cookie, id, `{"lang":"fr"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("lang=fr = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func postWritingOpening(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/opening", nil), cookie))
	return rec
}

// TestWritingOpening_SpeaksFirstAndIsIdempotent — the fix for the silent
// room, plus the guard that keeps a refresh from re-greeting her (and from
// paying for a second call to do it).
func TestWritingOpening_SpeaksFirstAndIsIdempotent(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		"你想写的是学校该不该禁手机。我们先把结构搭出来，再一块一块填。你自己更倾向哪一边？"))
	id := createWritingAtomHTTP(t, h, cookie, "该不该禁止学生带手机进校园。")

	rec := postWritingOpening(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("opening = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var first struct {
		Reply     string `json:"reply"`
		Generated bool   `json:"generated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode opening: %v — body=%s", err, rec.Body)
	}
	if strings.TrimSpace(first.Reply) == "" {
		t.Fatalf("opening was empty — the room is still silent")
	}
	if !first.Generated {
		t.Fatalf("first call should have generated the opening")
	}

	// Second call: same text, and NOT regenerated.
	rec2 := postWritingOpening(t, h, cookie, id)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second opening = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	var second struct {
		Reply     string `json:"reply"`
		Generated bool   `json:"generated"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if second.Generated {
		t.Fatalf("a refresh re-generated the opening — it must replay, not re-greet")
	}
	if second.Reply != first.Reply {
		t.Fatalf("replayed opening differs:\n first=%q\nsecond=%q", first.Reply, second.Reply)
	}
}

// TestWritingOpening_ModelFailureSurfaces — USER RULE (2026-08-22): an
// AI-dialogue failure is a real 502, never a canned stand-in. A hardcoded
// greeting here would be trivially easy and would lie about whether the coach
// works.
func TestWritingOpening_ModelFailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, streamErrorProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")
	if rec := postWritingOpening(t, h, cookie, id); rec.Code != http.StatusBadGateway {
		t.Fatalf("opening on model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}
