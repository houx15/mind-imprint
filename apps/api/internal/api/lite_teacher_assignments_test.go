package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

// assignJSON issues an authed request with an optional JSON body and decodes a
// 2xx response into out. Named apart from workspace_library_test.go's doJSON,
// which takes a string body and returns the recorder.
func assignJSON(t *testing.T, h http.Handler, c *http.Cookie, method, path string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, path, &buf), c))
	if out != nil && rec.Code < 300 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body)
		}
	}
	return rec.Code
}

func writingAssignmentBody(userIDs []string) map[string]any {
	return map[string]any{
		"kind": "writing", "title": "雨", "instructions": "",
		"payload": map[string]any{"prompt": "写一篇关于雨的记叙文", "targetWords": 800, "lang": "zh"},
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": userIDs,
	}
}

func TestTeacherCreatesAndListsAssignment(t *testing.T) {
	h, _, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{studentID.String()}), &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	var list struct {
		Assignments []struct {
			ID     string         `json:"id"`
			Counts map[string]int `json:"counts"`
		} `json:"assignments"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/assignments", &list)
	if len(list.Assignments) != 1 || list.Assignments[0].Counts["not_started"] != 1 || len(list.Assignments[0].Counts) != 5 {
		t.Fatalf("list = %+v", list)
	}

	var detail struct {
		Assignment struct {
			ID      string          `json:"id"`
			Payload json.RawMessage `json:"payload"`
		} `json:"assignment"`
		Recipients []struct {
			UserID      string  `json:"userId"`
			Status      string  `json:"status"`
			StatusLabel string  `json:"statusLabel"`
			AtomID      *string `json:"atomId"`
		} `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, &detail); code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	if len(detail.Recipients) != 1 || detail.Recipients[0].UserID != studentID.String() ||
		detail.Recipients[0].Status != "not_started" || detail.Recipients[0].StatusLabel != "未开始" ||
		detail.Recipients[0].AtomID != nil {
		t.Fatalf("detail recipients = %+v", detail.Recipients)
	}

	if code := assignJSON(t, h, teacher, "DELETE", "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("archive = %d", code)
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil); code != http.StatusNotFound {
		t.Fatalf("archived detail = %d, want 404", code)
	}
}

func TestTeacherAssignmentRejectsNonMember(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "as-stranger@demo.local")
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{stranger.String()}), nil); code != http.StatusBadRequest {
		t.Fatalf("stranger recipient = %d, want 400", code)
	}
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{}), nil); code != http.StatusBadRequest {
		t.Fatalf("no recipients = %d, want 400", code)
	}
}

func TestTeacherAssignmentOtherTeacher404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "as-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher detail = %d", code)
	}
	if code := assignJSON(t, h, other, "DELETE", "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher archive = %d", code)
	}
}

func TestTeacherAssignmentEditLocksAfterStart(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	aid := created.Assignment.ID

	// Before start: payload change allowed.
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"payload": map[string]any{"prompt": "写风", "targetWords": 600, "lang": "zh"}}, nil); code != http.StatusOK {
		t.Fatalf("pre-start payload patch = %d", code)
	}
	// Student starts.
	student := signInAs(t, pool, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusOK {
		t.Fatalf("start = %d", code)
	}
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"payload": map[string]any{"prompt": "写雪", "targetWords": 600, "lang": "zh"}}, nil); code != http.StatusConflict {
		t.Fatalf("post-start payload patch = %d, want 409", code)
	}
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"removeUserIds": []string{studentID.String()}}, nil); code != http.StatusConflict {
		t.Fatalf("remove started recipient = %d, want 409", code)
	}
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"title": "雨（改）", "dueAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339)}, nil); code != http.StatusOK {
		t.Fatalf("title/due patch after start = %d, want 200", code)
	}
}

// TestTeacherAssignmentPatchWaitsForStartInFlight: a start holds FOR SHARE on
// the assignment while it builds her item. A payload PATCH must wait for it,
// then see the started recipient and refuse.
func TestTeacherAssignmentPatchWaitsForStartInFlight(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	aid := created.Assignment.ID
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM lite_assignment WHERE id=$1 FOR SHARE`, aid).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE lite_assignment_recipient SET started_at = now() WHERE assignment_id = $1 AND user_id = $2`, aid, studentID); err != nil {
		t.Fatal(err)
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/lite/teacher/assignments/"+aid,
			bytes.NewBufferString(`{"payload":{"prompt":"写雪","targetWords":600,"lang":"zh"}}`)), teacher))
		done <- rec
	}()
	select {
	case rec := <-done:
		t.Fatalf("PATCH returned %d while a start held the assignment: body=%s", rec.Code, rec.Body)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var rec *httptest.ResponseRecorder
	select {
	case rec = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("PATCH did not return after the start committed")
	}
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != http.StatusConflict || e.Error.Code != "assignment_started" {
		t.Fatalf("PATCH after start = %d body=%s, want 409 assignment_started", rec.Code, rec.Body)
	}
}

// patchWhileTxHoldsLocks fires a PATCH while tx holds row locks, asserts it
// is still waiting after 300 ms, commits tx and returns the PATCH response.
func patchWhileTxHoldsLocks(t *testing.T, h http.Handler, teacher *http.Cookie, aid, body string, tx pgx.Tx) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.Background()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/lite/teacher/assignments/"+aid,
			bytes.NewBufferString(body)), teacher))
		done <- rec
	}()
	select {
	case rec := <-done:
		t.Fatalf("PATCH returned %d while the locks were held: body=%s", rec.Code, rec.Body)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case rec := <-done:
		return rec
	case <-time.After(10 * time.Second):
		t.Fatal("PATCH did not return after the locks were released")
	}
	return nil
}

// TestTeacherAssignmentPatchUsesLockedRow: a title-only PATCH whose loader
// read the old settings must not write them back over settings committed
// while it waited for the lock.
func TestTeacherAssignmentPatchUsesLockedRow(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	aid := created.Assignment.ID
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM lite_assignment WHERE id=$1 FOR UPDATE`, aid).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE lite_assignment SET payload = '{"prompt":"写风","targetWords":700,"lang":"zh"}' WHERE id=$1`, aid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE lite_assignment_recipient SET started_at = now() WHERE assignment_id = $1 AND user_id = $2`, aid, studentID); err != nil {
		t.Fatal(err)
	}

	rec := patchWhileTxHoldsLocks(t, h, teacher, aid, `{"title":"雨（改）"}`, tx)
	if rec.Code != http.StatusOK {
		t.Fatalf("title PATCH = %d body=%s, want 200", rec.Code, rec.Body)
	}
	var title, prompt string
	var target int
	if err := pool.QueryRow(ctx,
		`SELECT title, payload->>'prompt', (payload->>'targetWords')::int FROM lite_assignment WHERE id=$1`, aid).Scan(&title, &prompt, &target); err != nil {
		t.Fatal(err)
	}
	if title != "雨（改）" || prompt != "写风" || target != 700 {
		t.Fatalf("after PATCH: title=%q prompt=%q targetWords=%d, want 雨（改） 写风 700", title, prompt, target)
	}
}

// TestTeacherAssignmentRemoveRecipientDuringStartNoDeadlock: a start holds the
// assignment (FOR SHARE) then her recipient row (FOR UPDATE). A PATCH removing
// her takes the assignment first, so it waits instead of deadlocking, then
// refuses because she has started.
func TestTeacherAssignmentRemoveRecipientDuringStartNoDeadlock(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	aid := created.Assignment.ID
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM lite_assignment WHERE id=$1 FOR SHARE`, aid).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2 FOR UPDATE`, aid, studentID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE lite_assignment_recipient SET started_at = now() WHERE assignment_id = $1 AND user_id = $2`, aid, studentID); err != nil {
		t.Fatal(err)
	}

	rec := patchWhileTxHoldsLocks(t, h, teacher, aid, `{"removeUserIds":["`+studentID.String()+`"]}`, tx)
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != http.StatusConflict || e.Error.Code != "assignment_started" {
		t.Fatalf("remove during start = %d body=%s, want 409 assignment_started", rec.Code, rec.Body)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2`, aid, studentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recipient rows after refused removal = %d, want 1", n)
	}
}

// TestTeacherAssignmentPatchBeforeStart covers the PATCH paths that do not need
// a started recipient: a payload change, adding and removing a recipient, and
// the rejection codes.
func TestTeacherAssignmentPatchBeforeStart(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	second := createStudent(t, pool, SeedSchoolID, "as-second@demo.local")
	enrollStudent(t, pool, second, classID)
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	path := "/api/v1/lite/teacher/assignments/" + created.Assignment.ID

	var patched struct {
		Assignment struct {
			Title   string `json:"title"`
			Payload struct {
				Prompt string `json:"prompt"`
			} `json:"payload"`
		} `json:"assignment"`
	}
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{
		"title":         "风",
		"payload":       map[string]any{"prompt": "写风", "targetWords": 600, "lang": "zh"},
		"addUserIds":    []string{second.String()},
		"removeUserIds": []string{studentID.String()},
	}, &patched); code != http.StatusOK {
		t.Fatalf("patch = %d", code)
	}
	if patched.Assignment.Title != "风" || patched.Assignment.Payload.Prompt != "写风" {
		t.Fatalf("patched = %+v", patched)
	}
	var detail struct {
		Recipients []struct {
			UserID string `json:"userId"`
		} `json:"recipients"`
	}
	getJSON(t, h, teacher, path, &detail)
	if len(detail.Recipients) != 1 || detail.Recipients[0].UserID != second.String() {
		t.Fatalf("recipients after patch = %+v", detail.Recipients)
	}

	for name, body := range map[string]map[string]any{
		"empty title":      {"title": "  "},
		"bad due":          {"dueAt": "tomorrow"},
		"kind without fit": {"kind": "project"},
		"bad payload":      {"payload": map[string]any{"prompt": "写风", "targetWords": 600, "lang": "fr"}},
		"stranger added":   {"addUserIds": []string{createStudent(t, pool, SeedSchoolID, "as-stranger2@demo.local").String()}},
	} {
		if code := assignJSON(t, h, teacher, "PATCH", path, body, nil); code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, code)
		}
	}
}

func extractStub(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 10}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestExtractLiteAssignment(t *testing.T) {
	prov := &countingProvider{inner: extractStub(`{"prompt":"写一篇关于雨的记叙文","targetWords":600,"lang":"zh"}`)}
	h, pool, teacher, _, _ := liteTeacherFixtureWithProvider(t, prov)
	var got struct {
		Prompt      string `json:"prompt"`
		TargetWords *int   `json:"targetWords"`
		Lang        string `json:"lang"`
	}
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/assignments/extract",
		map[string]any{"text": "作业：写一篇关于雨的记叙文，600 字左右。"}, &got); code != http.StatusOK {
		t.Fatalf("extract = %d", code)
	}
	if got.Prompt != "写一篇关于雨的记叙文" || got.TargetWords == nil || *got.TargetWords != 600 || got.Lang != "zh" {
		t.Fatalf("extract = %+v", got)
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call l JOIN users u ON u.id = l.user_id
		 WHERE u.email = 'lt-teacher@demo.local' AND l.purpose = 'assignment_extract'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 || prov.count() != 1 {
		t.Fatalf("llm_call rows = %d, provider calls = %d; want 1, 1", n, prov.count())
	}
}

// TestExtractLiteAssignmentUnparsable: a reply that is not the JSON asked for
// surfaces as 502 and is still metered, since the tokens were spent.
func TestExtractLiteAssignmentUnparsable(t *testing.T) {
	prov := &countingProvider{inner: extractStub(`抱歉，我无法确定题目。`)}
	h, pool, teacher, _, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/lite/teacher/assignments/extract",
		bytes.NewBufferString(`{"text":"随便一段话"}`)), teacher))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unparsable extract = %d body=%s, want 502", rec.Code, rec.Body)
	}
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Error.Code != "ai_dialogue_failed" {
		t.Fatalf("code = %q body=%s", e.Error.Code, rec.Body)
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'assignment_extract'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("llm_call rows = %d, want 1", n)
	}

	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/assignments/extract",
		map[string]any{"text": "   "}, nil); code != http.StatusBadRequest {
		t.Fatalf("empty text = %d, want 400", code)
	}
}
