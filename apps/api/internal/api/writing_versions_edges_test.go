package api_test

// writing_versions_edges_test.go — edge cases from the Part A final review:
// the report title follows the submitted version (FR-2), discard on a
// finished writing with no version (FR-3b), and finish refusing a draft that
// became blank between its pre-check and its row lock (FR-4).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// writingReportTitleAndPiece reads a writing report without reaching a model.
// A stored report with prosePending makes the next GET run phase 2 (a model
// call); this fixture has no provider, and the prose is not what is under
// test, so the flag is cleared on the stored row first.
func writingReportTitleAndPiece(t *testing.T, h http.Handler, pool *pgxpool.Pool, c *http.Cookie, id string) (title, piece string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE atom_report SET report = report - 'prosePending' WHERE atom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/report", nil), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET report = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Report *struct {
			Title string `json:"title"`
			Piece string `json:"piece"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Report == nil {
		t.Fatalf("decode report: %v — body=%s", err, rec.Body)
	}
	return out.Report.Title, out.Report.Piece
}

// FR-2: a rename and an edit made while revising are not submitted, so the
// report keeps the latest version's title and body until 完成这篇. After the
// refinish both move together. With no version at all the live title is used.
func TestWritingVersionReportTitleFollowsLatestVersion(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	base := "/api/v1/writings/" + id
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if code := assignJSON(t, h, student, "PATCH", base, map[string]any{"title": "修改中的标题"}, nil); code != http.StatusOK {
		t.Fatalf("rename while revising = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文。"}, nil); code != http.StatusOK {
		t.Fatalf("draft while revising = %d", code)
	}

	title, piece := writingReportTitleAndPiece(t, h, pool, student, id)
	if title != "雨" || piece != "第一版正文。" {
		t.Fatalf("while revising: title=%q piece=%q, want the v1 title 雨 and body", title, piece)
	}

	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("refinish = %d", code)
	}
	title, piece = writingReportTitleAndPiece(t, h, pool, student, id)
	if title != "修改中的标题" || piece != "第二版正文。" {
		t.Fatalf("after refinish: title=%q piece=%q, want v2's title and body", title, piece)
	}

	// No version at all: the live title is what the report shows.
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM writing_version WHERE atom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE writing SET title = '没有版本时的标题' WHERE atom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if title, _ = writingReportTitleAndPiece(t, h, pool, student, id); title != "没有版本时的标题" {
		t.Fatalf("no version: title=%q, want the live title", title)
	}
}

// FR-3b: a finished writing with zero versions (finished by the old API in
// the deploy gap) can still leave revising. Discard clears revisingAt and
// keeps the draft and title as they are, instead of 404ing.
func TestWritingVersionDiscardWithoutVersion(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	base := "/api/v1/writings/" + id
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM writing_version WHERE atom_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "修改后的正文。"}, nil); code != http.StatusOK {
		t.Fatalf("draft while revising = %d", code)
	}

	var discarded struct {
		Title      string  `json:"title"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise/discard", nil, &discarded); code != http.StatusOK {
		t.Fatalf("discard with no version = %d, want 200", code)
	}
	if discarded.RevisingAt != nil || discarded.Title != "雨" {
		t.Fatalf("discard response = %+v", discarded)
	}
	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "修改后的正文。" {
		t.Fatalf("draft after discard = %q, want it left as it was", draft.Body)
	}
	if code, ec := writeErrorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "x"}); code != http.StatusForbidden || ec != "writing_finished" {
		t.Fatalf("draft after discard = %d %s, want 403 writing_finished", code, ec)
	}
}

// FR-4: finish's pre-check reads a non-blank draft, then an autosave of an
// empty body commits before finish takes the row lock. Finish must re-check
// the body under the lock and refuse with 400 missing_draft rather than
// insert an empty version (versions cannot be edited or deleted).
//
// The ordering is forced, not raced: this test holds the writing row lock
// (GetWritingForUpdate, the lock finish takes) in its own transaction and
// blanks the draft inside it without committing. Finish's pre-check reads
// the committed, non-blank draft, then blocks on the row lock. Only after
// that does the test commit.
func TestWritingVersionFinishRechecksBlankDraftUnderLock(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	atomID := uuid.MustParse(id)
	base := "/api/v1/writings/" + id

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := sqlc.New(pool).WithTx(tx)
	if _, err := qtx.GetWritingForUpdate(ctx, atomID); err != nil {
		t.Fatalf("GetWritingForUpdate: %v", err)
	}
	if _, err := qtx.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: atomID, Body: "   "}); err != nil {
		t.Fatalf("blank draft: %v", err)
	}

	type result struct {
		code int
		ec   string
	}
	done := make(chan result, 1)
	go func() {
		code, ec := writeErrorCode(t, h, student, "POST", base+"/finish", nil)
		done <- result{code, ec}
	}()

	select {
	case r := <-done:
		t.Fatalf("finish returned %d %s before the row lock was released — the ordering this test needs did not happen", r.code, r.ec)
	case <-time.After(500 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var got result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("finish never returned after the lock was released")
	}
	if got.code != http.StatusBadRequest || got.ec != "missing_draft" {
		t.Fatalf("finish = %d %s, want 400 missing_draft", got.code, got.ec)
	}
	if n := len(versionRows(t, pool, id)); n != 0 {
		t.Fatalf("versions = %d, want 0 (no empty version)", n)
	}
	var wr struct {
		Status string `json:"status"`
	}
	getJSON(t, h, student, base, &wr)
	if wr.Status == "finished" {
		t.Fatalf("status = finished, want the refused finish rolled back")
	}
}
