package api_test

// writing_assigned_lock_test.go — an assigned writing's language and target
// are the teacher's (owner, 2026-09-15). The student cannot change them from
// the setup dialog or the room's length meter. Her own writings behave as
// before.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// startAssignedWriting: a lite class, a writing assignment (zh, 800) and the
// student's started writing, which is still before its setup dialog.
func startAssignedWriting(t *testing.T) (http.Handler, *sqlc.Queries, *http.Cookie, string) {
	t.Helper()
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	return h, sqlc.New(pool), student, startAssignment(t, h, student, aid).AtomID
}

func putTargetWords(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/target-words", strings.NewReader(body)), cookie))
	return rec
}

func assertStoredTarget(t *testing.T, q *sqlc.Queries, id, lang string, words int32) {
	t.Helper()
	wr, err := q.GetWriting(context.Background(), mustUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if wr.Lang != lang || wr.TargetWords == nil || *wr.TargetWords != words {
		t.Fatalf("stored lang=%q targetWords=%v, want %q/%d", wr.Lang, wr.TargetWords, lang, words)
	}
}

func TestAssignedWritingSetupKeepsTeacherTarget(t *testing.T) {
	h, q, student, id := startAssignedWriting(t)

	const note = "我想写雨停之前那一刻的安静。"
	rec := putWritingSetup(t, h, student, id, `{"lang":"en","targetWords":300,"note":"`+note+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out writingSetupDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Lang != "zh" || out.TargetWords == nil || *out.TargetWords != 800 {
		t.Fatalf("returned lang=%q targetWords=%v, want the teacher's zh/800", out.Lang, out.TargetWords)
	}
	if out.SetupAt == nil || *out.SetupAt == "" {
		t.Fatalf("setupAt not stamped — the dialog would reopen on every visit")
	}
	assertStoredTarget(t, q, id, "zh", 800)

	msgs, err := q.ListAtomMessages(context.Background(), mustUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range msgs {
		if m.Role == "student" && m.Content == note {
			found = true
		}
	}
	if !found {
		t.Fatalf("the note was not saved as her turn; messages=%+v", msgs)
	}

	// Values the dialog never sends are ignored too, not refused: they are not
	// hers to set here.
	if rec := putWritingSetup(t, h, student, id, `{"lang":"fr","targetWords":0}`); rec.Code != http.StatusOK {
		t.Fatalf("setup with ignored fields = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	assertStoredTarget(t, q, id, "zh", 800)

	// Her own writing, same student: the body is applied as before.
	own := createWritingAtomHTTP(t, h, student, "我自己想写的一篇。")
	if rec := putWritingSetup(t, h, student, own, `{"lang":"en","targetWords":300}`); rec.Code != http.StatusOK {
		t.Fatalf("own setup = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	assertStoredTarget(t, q, own, "en", 300)
}

func TestAssignedWritingTargetWordsLocked(t *testing.T) {
	h, q, student, id := startAssignedWriting(t)

	rec := putTargetWords(t, h, student, id, `{"targetWords":300}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("target-words on an assigned writing = %d, want 409; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if body.Error.Code != "assigned_target_locked" || body.Error.Message != "作业的字数要求由老师设定" {
		t.Fatalf("error = %+v, want assigned_target_locked / 作业的字数要求由老师设定", body.Error)
	}
	assertStoredTarget(t, q, id, "zh", 800)

	own := createWritingAtomHTTP(t, h, student, "我自己想写的一篇。")
	if rec := putTargetWords(t, h, student, own, `{"targetWords":300}`); rec.Code != http.StatusOK {
		t.Fatalf("target-words on her own writing = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	wr, err := q.GetWriting(context.Background(), mustUUID(own))
	if err != nil || wr.TargetWords == nil || *wr.TargetWords != 300 {
		t.Fatalf("own writing targetWords = %v (err=%v), want 300", wr.TargetWords, err)
	}
}
