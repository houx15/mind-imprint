package api_test

import (
	"bytes"
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

type versionRow struct {
	Number      int
	Title       string
	Body        string
	WordCount   int
	SubmittedAt time.Time
}

// newBroughtWriting creates a writing she brought in with a finished body:
// it lands on the draft stage with writing_draft already filled.
func newBroughtWriting(t *testing.T, h http.Handler, c *http.Cookie, body string) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, c, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh", "body": body}, &created); code != http.StatusCreated {
		t.Fatalf("create writing = %d", code)
	}
	return created.ID
}

func versionRows(t *testing.T, pool *pgxpool.Pool, atomID string) []versionRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT number, title, body, word_count, submitted_at FROM writing_version WHERE atom_id = $1 ORDER BY number`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []versionRow
	for rows.Next() {
		var v versionRow
		if err := rows.Scan(&v.Number, &v.Title, &v.Body, &v.WordCount, &v.SubmittedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// startWritingHomework: the teacher assigns a writing to the fixture student,
// she starts it and writes a draft. Nothing is finished yet.
func startWritingHomework(t *testing.T, h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) (aid, atomID string, student *http.Cookie) {
	t.Helper()
	student = signInAs(t, pool, studentID)
	aid = createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	atomID = startAssignment(t, h, student, aid).AtomID
	if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": "雨下了一整天。"}, nil); code != http.StatusOK {
		t.Fatalf("put draft = %d", code)
	}
	return aid, atomID, student
}

func TestWritingVersionFirstFinishCreatesVersionOne(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "我读了 NASA 的报告，数据是 2024 年的。")

	var first struct {
		Status     string  `json:"status"`
		FinishedAt *string `json:"finishedAt"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &first); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if first.Status != "finished" || first.FinishedAt == nil || first.RevisingAt != nil {
		t.Fatalf("finish response = %+v", first)
	}
	got := versionRows(t, pool, id)
	if len(got) != 1 || got[0].Number != 1 || got[0].Title != "雨" ||
		got[0].Body != "我读了 NASA 的报告，数据是 2024 年的。" || got[0].WordCount != 15 {
		t.Fatalf("versions = %+v", got)
	}

	// A second finish while not revising changes nothing: no version 2,
	// finished_at stays where it was.
	var again struct {
		FinishedAt *string `json:"finishedAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &again); code != http.StatusOK {
		t.Fatalf("second finish = %d", code)
	}
	if n := len(versionRows(t, pool, id)); n != 1 {
		t.Fatalf("second finish added a version: %d rows", n)
	}
	if again.FinishedAt == nil || *again.FinishedAt != *first.FinishedAt {
		t.Fatalf("finishedAt moved: %v → %v", first.FinishedAt, again.FinishedAt)
	}
}

func TestWritingVersionFinishWithoutDraftAddsNothing(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh"}, &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+created.ID+"/finish", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("finish without draft = %d, want 400", code)
	}
	if n := len(versionRows(t, pool, created.ID)); n != 0 {
		t.Fatalf("versions = %d, want 0", n)
	}
}

// writeErrorCode is errorCode's sibling for endpoints under test in this
// file. Named distinctly (not errorCode) because reading_lens_branches_test.go
// already defines a same-package errorCode with a different signature
// (rec *httptest.ResponseRecorder, not a live request).
func writeErrorCode(t *testing.T, h http.Handler, c *http.Cookie, method, path string, body any) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, path, &buf), c))
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Error.Code
}

// Not homework: finished → writes refused (writing_finished); revise opens
// writes; discard restores the latest version and closes them again; finish
// while revising adds version 2.
func TestWritingVersionReviseDiscardAndRefinish(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	base := "/api/v1/writings/" + id
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}

	if code, ec := writeErrorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "改了"}); code != http.StatusForbidden || ec != "writing_finished" {
		t.Fatalf("draft on finished = %d %s, want 403 writing_finished", code, ec)
	}

	var revised struct {
		Status     string  `json:"status"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, &revised); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if revised.Status != "finished" || revised.RevisingAt == nil {
		t.Fatalf("revise response = %+v", revised)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文，还没提交。"}, nil); code != http.StatusOK {
		t.Fatalf("draft while revising = %d", code)
	}
	if code := assignJSON(t, h, student, "PATCH", base, map[string]any{"title": "新标题"}, nil); code != http.StatusOK {
		t.Fatalf("rename while revising = %d", code)
	}

	var discarded struct {
		Title      string  `json:"title"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise/discard", nil, &discarded); code != http.StatusOK {
		t.Fatalf("discard = %d", code)
	}
	if discarded.RevisingAt != nil || discarded.Title != "雨" {
		t.Fatalf("discard response = %+v", discarded)
	}
	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "第一版正文。" {
		t.Fatalf("draft after discard = %q", draft.Body)
	}
	if code, ec := writeErrorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "x"}); code != http.StatusForbidden || ec != "writing_finished" {
		t.Fatalf("draft after discard = %d %s", code, ec)
	}

	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("second revise = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文。"}, nil); code != http.StatusOK {
		t.Fatalf("draft before refinish = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("refinish = %d", code)
	}
	got := versionRows(t, pool, id)
	if len(got) != 2 || got[1].Number != 2 || got[1].Body != "第二版正文。" || got[0].Body != "第一版正文。" {
		t.Fatalf("versions = %+v", got)
	}
	var wr struct {
		RevisingAt *string `json:"revisingAt"`
	}
	getJSON(t, h, student, base, &wr)
	if wr.RevisingAt != nil {
		t.Fatalf("revisingAt after refinish = %v", *wr.RevisingAt)
	}
}

func TestWritingVersionReviseRefusesUnfinished(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "正文。")
	if code, ec := writeErrorCode(t, h, student, "POST", "/api/v1/writings/"+id+"/revise", nil); code != http.StatusBadRequest || ec != "writing_not_finished" {
		t.Fatalf("revise unfinished = %d %s", code, ec)
	}
}

// Homework past its deadline, submitted, being revised: every write is
// writing_locked, and the unsubmitted draft stays where it is.
func TestWritingVersionLockedHomework(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	base := "/api/v1/writings/" + atomID
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise before due = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "没提交的修改。"}, nil); code != http.StatusOK {
		t.Fatalf("draft before due = %d", code)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 minute' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{"PUT", base + "/draft"}, {"POST", base + "/finish"}, {"POST", base + "/revise"}, {"POST", base + "/revise/discard"},
	} {
		if code, ec := writeErrorCode(t, h, student, c.method, c.path, map[string]any{"body": "x"}); code != http.StatusForbidden || ec != "writing_locked" {
			t.Fatalf("%s %s after due = %d %s, want 403 writing_locked", c.method, c.path, code, ec)
		}
	}
	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "没提交的修改。" {
		t.Fatalf("unsubmitted draft = %q, want it kept", draft.Body)
	}
}

// Task 4 ruling 5: finish → revise → edit the draft → finish again adds
// version 2 with the NEW body, clears revisingAt, and leaves finishedAt
// where the first finish put it (a refinish is a new version, not a new
// "when did she first finish").
func TestWritingVersionRefinishUsesEditedDraft(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	base := "/api/v1/writings/" + id

	var first struct {
		FinishedAt *string `json:"finishedAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, &first); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if first.FinishedAt == nil {
		t.Fatalf("first finish has no finishedAt")
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文，改过了。"}, nil); code != http.StatusOK {
		t.Fatalf("edit draft while revising = %d", code)
	}

	var second struct {
		FinishedAt *string `json:"finishedAt"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, &second); code != http.StatusOK {
		t.Fatalf("second finish = %d", code)
	}
	if second.RevisingAt != nil {
		t.Fatalf("revisingAt after second finish = %v, want cleared", *second.RevisingAt)
	}
	if second.FinishedAt == nil || *second.FinishedAt != *first.FinishedAt {
		t.Fatalf("finishedAt moved on refinish: %v → %v", first.FinishedAt, second.FinishedAt)
	}

	got := versionRows(t, pool, id)
	if len(got) != 2 {
		t.Fatalf("versions = %d, want 2: %+v", len(got), got)
	}
	if got[0].Number != 1 || got[0].Body != "第一版正文。" {
		t.Fatalf("version 1 = %+v", got[0])
	}
	if got[1].Number != 2 || got[1].Body != "第二版正文，改过了。" {
		t.Fatalf("version 2 = %+v, want the edited body", got[1])
	}
}

// TestWritingVersionDraftPutSerializesWithDiscard pins the fix-round-1
// finding: PUT /draft must not be able to commit between discard's restore
// and discard's own commit. It reproduces exactly that ordering — not with
// a sleep-and-hope race, but by holding the SAME row lock
// (GetWritingForUpdate, "FOR UPDATE") discardWritingRevision takes, in a
// transaction this test controls directly, and proving a concurrent PUT
// /draft genuinely blocks on it at the database level (Postgres, not a
// goroutine scheduling accident) until that transaction commits.
//
// The one inherently timing-based part is the "still blocked" check midway
// through — bounded by a generous 500ms against a lock wait that has no
// legitimate way to resolve early, so it is not expected to be flaky. The
// assertion that actually proves the fix is fully deterministic: it runs
// only after the discard transaction has committed, and checks that the
// concurrent PUT (a) was refused 403 writing_finished (not a stale success)
// and (b) never got to overwrite the restored draft.
func TestWritingVersionDraftPutSerializesWithDiscard(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	atomID := uuid.MustParse(id)
	base := "/api/v1/writings/" + id

	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	// A legitimate edit while revising, BEFORE the discard-in-flight window
	// below — this is what discard is about to overwrite.
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "修改中，还没保存的版本。"}, nil); code != http.StatusOK {
		t.Fatalf("draft while revising = %d", code)
	}

	ctx := context.Background()
	q := sqlc.New(pool)

	// Hold the exact lock discardWritingRevision takes, in a transaction this
	// test controls, so it can pause discard's effect mid-flight.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin discard tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed below
	qtx := q.WithTx(tx)
	wr, err := qtx.GetWritingForUpdate(ctx, atomID)
	if err != nil {
		t.Fatalf("GetWritingForUpdate: %v", err)
	}
	if !wr.RevisingAt.Valid {
		t.Fatalf("not revising: %+v", wr)
	}

	// A PUT /draft racing the discard: its own GetWritingForUpdate must block
	// on the row lock this test's transaction is holding.
	type raceResult struct {
		code int
		ec   string
	}
	done := make(chan raceResult, 1)
	go func() {
		code, ec := writeErrorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "赛跑写入，不该落地。"})
		done <- raceResult{code, ec}
	}()

	select {
	case <-done:
		t.Fatalf("PUT /draft returned before the discard transaction (holding the row lock) committed — the write is not serializing with discard")
	case <-time.After(500 * time.Millisecond):
		// Still blocked on the lock, as it must be.
	}

	// Now perform discard's own effect — restore the draft and title from
	// the latest version, clear revising_at — and commit, releasing the lock
	// the racing PUT is waiting on.
	latest, err := qtx.GetLatestWritingVersion(ctx, atomID)
	if err != nil {
		t.Fatalf("GetLatestWritingVersion: %v", err)
	}
	if _, err := qtx.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: atomID, Body: latest.Body}); err != nil {
		t.Fatalf("restore draft: %v", err)
	}
	if err := qtx.RenameWriting(ctx, sqlc.RenameWritingParams{AtomID: atomID, Title: latest.Title}); err != nil {
		t.Fatalf("restore title: %v", err)
	}
	if err := qtx.ClearWritingRevising(ctx, atomID); err != nil {
		t.Fatalf("clear revising: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit discard tx: %v", err)
	}

	var race raceResult
	select {
	case race = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PUT /draft never returned after the discard transaction committed")
	}
	if race.code != http.StatusForbidden || race.ec != "writing_finished" {
		t.Fatalf("racing PUT /draft = %d %s, want 403 writing_finished (discard's restore must win)", race.code, race.ec)
	}

	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "第一版正文。" {
		t.Fatalf("draft after discard = %q, want the restored version body, not the racing write", draft.Body)
	}
}

// A homework she never submitted stays submittable after the deadline.
func TestWritingVersionLateFirstSubmissionIsNotLocked(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("late first finish = %d, want 200", code)
	}
	if n := len(versionRows(t, pool, atomID)); n != 1 {
		t.Fatalf("versions = %d, want 1", n)
	}
}
