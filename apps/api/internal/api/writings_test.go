package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// createWritingAtomHTTP returns a new writing's atom id via the real
// POST /api/v1/writings route. Named distinctly from createWritingAtom
// (atom_loader_test.go, Task 1.5's store-level helper of the same idea, from
// before this route existed) to avoid a redeclaration — shared by Tasks 3-7.
func createWritingAtomHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, idea string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"idea":"` + idea + `","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create writing = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

// TestCreateWriting_IdeaBecomesTitleAndFirstMessage — the sentence she typed
// into the box does double duty: truncated it is the title, verbatim it is
// atom_message seq=1 role='student' — because 先聊's first line really is the
// one she just said, and it must not vanish from the transcript.
func TestCreateWriting_IdeaBecomesTitleAndFirstMessage(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "我想写中国的可持续发展")

	a, err := q.GetAtom(t.Context(), mustUUID(id))
	if err != nil {
		t.Fatalf("GetAtom: %v", err)
	}
	if a.Kind != "writing" {
		t.Fatalf("atom kind = %q, want \"writing\"", a.Kind)
	}

	wr, err := q.GetWriting(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("GetWriting: %v", err)
	}
	if wr.Title != "我想写中国的可持续发展" {
		t.Fatalf("title = %q, want the idea verbatim", wr.Title)
	}
	if wr.Stage != "outline" {
		t.Fatalf("stage = %q, want \"outline\"", wr.Stage)
	}
	if wr.TargetWords != nil {
		t.Fatalf("targetWords = %v, want nil on a new writing", wr.TargetWords)
	}

	msgs, err := q.ListAtomMessages(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d atom_messages, want 1", len(msgs))
	}
	if msgs[0].Role != "student" || msgs[0].Content != "我想写中国的可持续发展" {
		t.Fatalf("first message = role=%q content=%q, want role=student content=idea", msgs[0].Role, msgs[0].Content)
	}
}

// TestCreateWriting_TitleTruncatedTo200Runes — the title is a truncated
// projection of the idea; the message stays the full, untruncated sentence.
func TestCreateWriting_TitleTruncatedTo200Runes(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	long := strings.Repeat("思", 250)

	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"idea": long, "lang": "zh"})
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings", strings.NewReader(string(body))), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create writing = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}

	wr, err := q.GetWriting(t.Context(), mustUUID(out.ID))
	if err != nil {
		t.Fatalf("GetWriting: %v", err)
	}
	if got := len([]rune(wr.Title)); got != 200 {
		t.Fatalf("title rune length = %d, want 200", got)
	}

	msgs, err := q.ListAtomMessages(t.Context(), mustUUID(out.ID))
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	if len(msgs) != 1 || len([]rune(msgs[0].Content)) != 250 {
		t.Fatalf("first message should carry the FULL idea untruncated, got %d runes", len([]rune(msgs[0].Content)))
	}
}

// TestCreateWriting_EmptyIdeaIs400 — unlike reading, a writing has nothing to
// talk about without an idea, so an empty one is refused outright.
func TestCreateWriting_EmptyIdeaIs400(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"idea":"  ","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings", body), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty idea = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "missing_idea") {
		t.Fatalf("want missing_idea, got %s", rec.Body)
	}
}

func TestListWritings_OnlyMineNewestFirst(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	first := createWritingAtomHTTP(t, h, cookie, "第一个想法")
	second := createWritingAtomHTTP(t, h, cookie, "第二个想法")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /writings = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Writings []struct {
			ID          string `json:"id"`
			Stage       string `json:"stage"`
			TargetWords *int32 `json:"targetWords"`
		} `json:"writings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Writings) != 2 {
		t.Fatalf("got %d writings, want 2", len(out.Writings))
	}
	if out.Writings[0].ID != second || out.Writings[1].ID != first {
		t.Fatalf("order = [%s,%s], want newest first [%s,%s]", out.Writings[0].ID, out.Writings[1].ID, second, first)
	}
	if out.Writings[0].Stage != "outline" {
		t.Fatalf("stage = %q, want outline", out.Writings[0].Stage)
	}
	if out.Writings[0].TargetWords != nil {
		t.Fatalf("targetWords = %v, want null", out.Writings[0].TargetWords)
	}
}

// TestListWritings_OnlyCallersOwn — another student's writing must not leak
// into this caller's list.
func TestListWritings_OnlyCallersOwn(t *testing.T) {
	h, cookie, q, pool := liteHandler(t)
	createWritingAtomHTTP(t, h, cookie, "我自己的想法")

	otherID := createStudent(t, pool, SeedSchoolID, "other-writer@demo.local")
	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "writing", UserID: otherID})
	if err != nil {
		t.Fatalf("create other's atom: %v", err)
	}
	if _, err := q.CreateWriting(context.Background(), sqlc.CreateWritingParams{
		AtomID: at.ID, Title: "别人的想法", Lang: "zh",
	}); err != nil {
		t.Fatalf("create other's writing: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /writings = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Writings []struct {
			ID string `json:"id"`
		} `json:"writings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Writings) != 1 {
		t.Fatalf("got %d writings, want 1 (only caller's own)", len(out.Writings))
	}
}

func TestGetWriting_UnknownIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/writings/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown writing = %d, want 404", rec.Code)
	}
}

func TestRenameWriting(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "原来的想法")

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"新标题"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/writings/"+id, body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Title != "新标题" {
		t.Fatalf("title = %q, want 新标题 (err=%v)", out.Title, err)
	}
}

// TestWritingCrossFormIsolation — a writing-form atom accessed via
// /readings/{id} → 404, and a reading-form atom accessed via /writings/{id}
// → 404. Same invariant as P1 Task 3's wrong-kind assertion
// (TestLoadOwnedReadingAtom_WrongKindIs404), checked from both directions.
func TestWritingCrossFormIsolation(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)

	writingID := createWritingAtomHTTP(t, h, cookie, "一个写作想法")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+writingID, nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET reading route on a writing atom = %d, want 404; body=%s", rec.Code, rec.Body)
	}

	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "reading", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("create reading atom: %v", err)
	}
	if _, err := q.CreateReading(context.Background(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: "一篇文章", Lang: "zh",
	}); err != nil {
		t.Fatalf("create reading: %v", err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+at.ID.String(), nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET writing route on a reading atom = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body missing not_found code — got %s", rec.Body)
	}
}
