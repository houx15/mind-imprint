package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

func createAssignment(t *testing.T, h http.Handler, teacher *http.Cookie, classID string, body map[string]any) string {
	t.Helper()
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", body, &created); code != http.StatusCreated {
		t.Fatalf("create assignment = %d", code)
	}
	return created.Assignment.ID
}

func readingAssignmentBody(title string, payload map[string]any, userIDs []string) map[string]any {
	return map[string]any{
		"kind": "reading", "title": title, "instructions": "",
		"payload": payload,
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": userIDs,
	}
}

type startResp struct {
	Kind      string  `json:"kind"`
	AtomID    string  `json:"atomId"`
	ProjectID *string `json:"projectId"`
}

func startAssignment(t *testing.T, h http.Handler, c *http.Cookie, aid string) startResp {
	t.Helper()
	var out startResp
	if code := assignJSON(t, h, c, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, &out); code != http.StatusOK {
		t.Fatalf("start %s = %d", aid, code)
	}
	return out
}

// libraryArticleWithTier returns a library article that has the given tier.
func libraryArticleWithTier(t *testing.T, tier int) library.Article {
	t.Helper()
	for _, art := range library.All() {
		if _, ok := art.LevelAt(tier); ok {
			return art
		}
	}
	t.Fatalf("no library article has tier %d", tier)
	return library.Article{}
}

func TestInboxShowsUnreadThenSeen(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	type inboxResp struct {
		Items []struct {
			Type        string  `json:"type"`
			ID          string  `json:"id"`
			Kind        string  `json:"kind"`
			Title       string  `json:"title"`
			ClassName   string  `json:"className"`
			Unread      bool    `json:"unread"`
			Status      string  `json:"status"`
			StatusLabel string  `json:"statusLabel"`
			AtomID      *string `json:"atomId"`
		} `json:"items"`
		Unread int `json:"unread"`
	}
	var inbox inboxResp
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 1 || len(inbox.Items) != 1 {
		t.Fatalf("inbox = %+v", inbox)
	}
	it := inbox.Items[0]
	if it.Type != "assignment" || it.ID != aid || it.Kind != "writing" || it.Title != "雨" || it.ClassName != "Lite Class" ||
		!it.Unread || it.Status != "not_started" || it.StatusLabel != "未开始" || it.AtomID != nil {
		t.Fatalf("inbox item = %+v", it)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/seen", nil, nil); code != http.StatusNoContent {
		t.Fatalf("seen = %d", code)
	}
	inbox = inboxResp{}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 0 || len(inbox.Items) != 1 || inbox.Items[0].Unread {
		t.Fatalf("inbox after seen = %+v", inbox)
	}

	started := startAssignment(t, h, student, aid)
	inbox = inboxResp{}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Items[0].Status != "in_progress" || inbox.Items[0].AtomID == nil || *inbox.Items[0].AtomID != started.AtomID {
		t.Fatalf("inbox after start = %+v", inbox.Items[0])
	}

	// A student who is not a recipient cannot mark it seen.
	other := createStudent(t, pool, SeedSchoolID, "as-seen-other@demo.local")
	if code := assignJSON(t, h, signInAs(t, pool, other), "POST", "/api/v1/lite/assignments/"+aid+"/seen", nil, nil); code != http.StatusNotFound {
		t.Fatalf("non-recipient seen = %d, want 404", code)
	}
}

// TestInboxOnlyAssignmentsWithPlan2Keys: the inbox lists only assignments,
// even when parent reports exist for her (there is no parent report delivery
// to the student). An assignment item keeps plan 2's key set exactly: an
// unstarted assignment with no instructions still carries "instructions" and
// "atomId".
func TestInboxOnlyAssignmentsWithPlan2Keys(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	student := signInAs(t, pool, studentID)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	generateParentReport(t, h, teacher, classID, studentID)

	var out struct {
		Items  []map[string]any `json:"items"`
		Unread int              `json:"unread"`
	}
	if code := getJSON(t, h, student, "/api/v1/lite/inbox", &out); code != http.StatusOK {
		t.Fatalf("inbox = %d", code)
	}
	if out.Unread != 1 || len(out.Items) != 1 || out.Items[0]["type"] != "assignment" || out.Items[0]["id"] != aid {
		t.Fatalf("inbox = %d %+v, want only the assignment", out.Unread, out.Items)
	}
	wantAssignKeys := []string{"atomId", "className", "dueAt", "id", "instructions", "kind", "returnDueAt", "returnNote", "status", "statusLabel", "title", "type", "unread"}
	if got := keysOf(out.Items[0]); !reflect.DeepEqual(got, wantAssignKeys) {
		t.Fatalf("assignment keys = %v, want %v", got, wantAssignKeys)
	}
	if out.Items[0]["instructions"] != "" || out.Items[0]["atomId"] != nil || out.Items[0]["kind"] != "writing" || out.Items[0]["status"] != "not_started" {
		t.Fatalf("assignment item = %+v", out.Items[0])
	}
}

func TestStartIsIdempotentUnderConcurrency(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	type result struct {
		code int
		body []byte
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/lite/assignments/"+aid+"/start", nil), student))
			results <- result{rec.Code, rec.Body.Bytes()}
		}()
	}
	var ids []string
	for i := 0; i < 2; i++ {
		res := <-results
		var out startResp
		if res.code != http.StatusOK || json.Unmarshal(res.body, &out) != nil {
			t.Fatalf("concurrent start = %d body=%s", res.code, res.body)
		}
		ids = append(ids, out.AtomID)
	}
	if ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("concurrent starts returned %q and %q", ids[0], ids[1])
	}
	if again := startAssignment(t, h, student, aid); again.AtomID != ids[0] || again.Kind != "writing" || again.ProjectID != nil {
		t.Fatalf("repeat start = %+v, want atom %s", again, ids[0])
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM atom WHERE user_id=$1 AND kind='writing'`, studentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("writings created = %d, want 1", n)
	}
	// The title is the assignment's; the prompt is stored as the teacher's, on
	// the writing itself, and never as a message of hers.
	var title string
	var target *int32
	var prompt *string
	if err := pool.QueryRow(context.Background(), `SELECT title, target_words, assigned_prompt FROM writing WHERE atom_id=$1`, ids[0]).Scan(&title, &target, &prompt); err != nil {
		t.Fatal(err)
	}
	if title != "雨" || target == nil || *target != 800 || prompt == nil || *prompt != "写一篇关于雨的记叙文" {
		t.Fatalf("writing title=%q target=%v assigned_prompt=%v, want 雨 800 写一篇关于雨的记叙文", title, target, prompt)
	}
	var msgs int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM atom_message WHERE atom_id=$1`, ids[0]).Scan(&msgs); err != nil {
		t.Fatal(err)
	}
	if msgs != 0 {
		t.Fatalf("atom_message rows after an assigned writing start = %d, want 0", msgs)
	}
	var dto struct {
		AssignedPrompt *string `json:"assignedPrompt"`
	}
	if code := getJSON(t, h, student, "/api/v1/writings/"+ids[0], &dto); code != http.StatusOK || dto.AssignedPrompt == nil || *dto.AssignedPrompt != "写一篇关于雨的记叙文" {
		t.Fatalf("writing DTO = %d assignedPrompt=%v", code, dto.AssignedPrompt)
	}
}

func TestStartProjectBypassesHomepageGate(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	// The gate is closed for a fresh student.
	if code := assignJSON(t, h, student, "POST", "/api/v1/pbl/projects", map[string]any{"idea": "x"}, nil); code != http.StatusConflict {
		t.Fatalf("own project = %d, want 409 (gate)", code)
	}
	aid := createAssignment(t, h, teacher, classID, map[string]any{
		"kind": "project", "title": "杯子", "instructions": "",
		"payload": map[string]any{"drivingQuestion": "怎样让校园少用一次性杯子？", "description": "先统计一周食堂用掉的杯子"},
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": []string{studentID.String()},
	})
	out := startAssignment(t, h, student, aid)
	if out.Kind != "project" || out.ProjectID == nil || *out.ProjectID != out.AtomID {
		t.Fatalf("start project = %+v", out)
	}
	var idea, name string
	var assigned bool
	var brief *string
	if err := pool.QueryRow(context.Background(), `SELECT idea, name, assigned, assigned_brief FROM pbl_project WHERE atom_id=$1`, out.AtomID).Scan(&idea, &name, &assigned, &brief); err != nil {
		t.Fatal(err)
	}
	if idea != "怎样让校园少用一次性杯子？" || name != "杯子" || !assigned || brief == nil || *brief != "先统计一周食堂用掉的杯子" {
		t.Fatalf("project idea=%q name=%q assigned=%v brief=%v", idea, name, assigned, brief)
	}
	var list []struct {
		ID       string `json:"id"`
		Assigned bool   `json:"assigned"`
	}
	if code := getJSON(t, h, student, "/api/v1/pbl/projects", &list); code != http.StatusOK || len(list) != 1 || list[0].ID != out.AtomID || !list[0].Assigned {
		t.Fatalf("project list = %d %+v, want the assigned project", code, list)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/pbl/projects", map[string]any{"idea": "y"}, nil); code != http.StatusConflict {
		t.Fatalf("own project after assigned start = %d, want still 409", code)
	}
}

func TestStartLibraryNullTierUsesSuggestion(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	want := library.SuggestTier(0, 0)
	art := libraryArticleWithTier(t, want)
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("库里一篇",
		map[string]any{"source": "library", "slug": art.Slug}, []string{studentID.String()}))
	out := startAssignment(t, h, signInAs(t, pool, studentID), aid)
	var slug string
	var tier int
	if err := pool.QueryRow(context.Background(),
		`SELECT library_slug, library_tier FROM reading WHERE atom_id=$1`, out.AtomID).Scan(&slug, &tier); err != nil {
		t.Fatal(err)
	}
	if out.Kind != "reading" || slug != art.Slug || tier != want {
		t.Fatalf("started reading = %+v slug=%q tier=%d, want %q tier %d", out, slug, tier, art.Slug, want)
	}
}

// TestStartLibraryResumeRules: an unfinished reading she opened herself is
// reused; one already linked to another assignment is not.
func TestStartLibraryResumeRules(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	art := library.All()[0]
	tier := art.Levels[0].Tier
	payload := map[string]any{"source": "library", "slug": art.Slug, "tier": tier}

	var own struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/library/"+art.Slug+"/levels/"+strconv.Itoa(tier), nil, &own); code != http.StatusCreated {
		t.Fatalf("own library start = %d", code)
	}
	first := startAssignment(t, h, student, createAssignment(t, h, teacher, classID,
		readingAssignmentBody("第一份", payload, []string{studentID.String()})))
	if first.AtomID != own.ID {
		t.Fatalf("first assigned start = %s, want her own unfinished reading %s", first.AtomID, own.ID)
	}
	second := startAssignment(t, h, student, createAssignment(t, h, teacher, classID,
		readingAssignmentBody("第二份", payload, []string{studentID.String()})))
	if second.AtomID == first.AtomID {
		t.Fatalf("second assignment reused an atom already linked to the first: %s", second.AtomID)
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM lite_assignment_recipient WHERE user_id=$1 AND atom_id IS NOT NULL`, studentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("linked recipients = %d, want 2", n)
	}
}

func TestStartReadingTextUsesTextLanguage(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("雨的文章",
		map[string]any{"source": "text", "text": "The rain came.\n\nIt did not stop."}, []string{studentID.String()}))
	out := startAssignment(t, h, signInAs(t, pool, studentID), aid)
	var title, lang, body string
	if err := pool.QueryRow(context.Background(),
		`SELECT r.title, r.lang, s.body FROM reading r JOIN reading_source s ON s.atom_id = r.atom_id WHERE r.atom_id=$1`,
		out.AtomID).Scan(&title, &lang, &body); err != nil {
		t.Fatal(err)
	}
	if title != "雨的文章" || lang != "en" || !strings.Contains(body, "It did not stop.") {
		t.Fatalf("reading title=%q lang=%q body=%q", title, lang, body)
	}
}

func TestStartReadingURLFetchFailedHidesCause(t *testing.T) {
	_, pool, teacher, classID, studentID := liteTeacherFixture(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Fetcher: errFetcher{},
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("Rain",
		map[string]any{"source": "url", "url": "https://example.com/rain"}, []string{studentID.String()}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/lite/assignments/"+aid+"/start", bytes.NewReader(nil)), signInAs(t, pool, studentID)))
	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != http.StatusBadRequest || e.Error.Code != "fetch_failed" || e.Error.Message != "开始失败：链接无法读取正文，请告知老师更换阅读材料" {
		t.Fatalf("fetch failure = %d body=%s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "simulated") {
		t.Fatalf("response leaked the fetcher error: %s", rec.Body)
	}
	assertNotStarted(t, pool, aid, studentID)
}

// enArticleFetcher serves one English article. It embeds errFetcher for the
// Fetcher's other methods.
type enArticleFetcher struct{ errFetcher }

func (enArticleFetcher) FetchReadable(ctx context.Context, rawURL string) (string, string, *materialize.DOIMeta, error) {
	return "Why the Rain Stays",
		"The rain came in the night and stayed for eleven days.\n\nNobody in the town could remember a week like it.",
		nil, nil
}

// TestStartReadingURLUsesFetchedTitleAndLang: the assignment's title is the
// teacher's name for the practice. The article keeps the page's own title, and
// its language comes from the page's text.
func TestStartReadingURLUsesFetchedTitleAndLang(t *testing.T) {
	_, pool, teacher, classID, studentID := liteTeacherFixture(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Fetcher: enArticleFetcher{},
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("第三周阅读",
		map[string]any{"source": "url", "url": "https://example.com/rain"}, []string{studentID.String()}))
	out := startAssignment(t, h, signInAs(t, pool, studentID), aid)
	var title, lang, srcTitle string
	var srcURL *string
	if err := pool.QueryRow(context.Background(),
		`SELECT r.title, r.lang, s.title, s.source_url FROM reading r JOIN reading_source s ON s.atom_id = r.atom_id WHERE r.atom_id=$1`,
		out.AtomID).Scan(&title, &lang, &srcTitle, &srcURL); err != nil {
		t.Fatal(err)
	}
	if title != "Why the Rain Stays" || srcTitle != "Why the Rain Stays" || lang != "en" || srcURL == nil || *srcURL != "https://example.com/rain" {
		t.Fatalf("reading title=%q source title=%q lang=%q url=%v", title, srcTitle, lang, srcURL)
	}
}

// TestStartsOutnumberingThePool: a class pressing 开始 at once, with more starts
// in flight than the pool has connections (18 starts, MaxConns 3). A start
// holds at most one connection at a time, so every one finishes. A start that
// kept its locked connection while a helper waited for a second one would
// starve the pool and hang here.
func TestStartsOutnumberingThePool(t *testing.T) {
	_, pool, teacher, classID, firstStudent := liteTeacherFixture(t)
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 3
	small, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(small.Close)
	h := New(Deps{
		Queries: sqlc.New(small), Pool: small,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()

	students := []uuid.UUID{firstStudent}
	for i := 1; i < 6; i++ {
		id := createStudent(t, pool, SeedSchoolID, fmt.Sprintf("as-burst-%d@demo.local", i))
		enrollStudent(t, pool, id, classID)
		students = append(students, id)
	}
	userIDs := make([]string, 0, len(students))
	cookies := make([]*http.Cookie, 0, len(students))
	for _, s := range students {
		userIDs = append(userIDs, s.String())
		cookies = append(cookies, signInAs(t, pool, s))
	}
	art := library.All()[0]
	aids := []string{
		createAssignment(t, h, teacher, classID, writingAssignmentBody(userIDs)),
		createAssignment(t, h, teacher, classID, map[string]any{
			"kind": "project", "title": "杯子", "instructions": "",
			"payload": map[string]any{"drivingQuestion": "怎样让校园少用一次性杯子？"},
			"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
			"userIds": userIDs,
		}),
		createAssignment(t, h, teacher, classID, readingAssignmentBody("库里一篇",
			map[string]any{"source": "library", "slug": art.Slug, "tier": art.Levels[0].Tier}, userIDs)),
	}

	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	type result struct {
		code int
		body string
	}
	total := len(cookies) * len(aids)
	results := make(chan result, total)
	for _, c := range cookies {
		for _, aid := range aids {
			go func(c *http.Cookie, aid string) {
				rec := httptest.NewRecorder()
				req := withCookie(httptest.NewRequest("POST", "/api/v1/lite/assignments/"+aid+"/start", nil), c).WithContext(deadline)
				h.ServeHTTP(rec, req)
				results <- result{rec.Code, rec.Body.String()}
			}(c, aid)
		}
	}
	for i := 0; i < total; i++ {
		select {
		case res := <-results:
			if res.code != http.StatusOK {
				t.Errorf("start = %d body=%s", res.code, res.body)
			}
		case <-deadline.Done():
			t.Fatalf("starts still waiting after 20s: %d of %d returned", i, total)
		}
	}
	var linked int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM lite_assignment_recipient WHERE atom_id IS NOT NULL AND started_at IS NOT NULL AND assignment_id = ANY($1::uuid[])`,
		aids).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != total {
		t.Fatalf("started recipients = %d, want %d", linked, total)
	}
}

// TestStartRefusesWhenAssignmentChangedAfterPreflight: the teacher's edit
// commits after the start's unlocked preflight read the settings and before it
// takes its locks. The start must not build her item from the settings it read
// first. The seam is startAfterPreflightHook, which runs in exactly that window.
func TestStartRefusesWhenAssignmentChangedAfterPreflight(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	ctx := context.Background()

	restore := SetStartAfterPreflightHookForTest(func() {
		if _, err := pool.Exec(ctx,
			`UPDATE lite_assignment SET payload = '{"prompt":"写雪","targetWords":600,"lang":"zh"}' WHERE id=$1`, aid); err != nil {
			t.Error(err)
		}
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/lite/assignments/"+aid+"/start", nil), student))
	restore()

	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != http.StatusConflict || e.Error.Code != "assignment_changed" || e.Error.Message != "开始失败：老师刚修改了这份作业，请重新开始" {
		t.Fatalf("start after a mid-start edit = %d body=%s, want 409 assignment_changed", rec.Code, rec.Body)
	}
	assertNotStarted(t, pool, aid, studentID)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM atom WHERE user_id=$1 AND kind='writing'`, studentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("writings after a refused start = %d, want 0", n)
	}

	// Pressing 开始 again builds her writing from the new settings.
	started := startAssignment(t, h, student, aid)
	var prompt *string
	if err := pool.QueryRow(ctx, `SELECT assigned_prompt FROM writing WHERE atom_id=$1`, started.AtomID).Scan(&prompt); err != nil {
		t.Fatal(err)
	}
	if prompt == nil || *prompt != "写雪" {
		t.Fatalf("retried start assigned_prompt = %v, want 写雪", prompt)
	}
}

// TestStartAndInboxRequireCurrentEnrollment: a student removed from the class
// keeps her recipient row, but the assignment is no longer hers to start or to
// see.
func TestStartAndInboxRequireCurrentEnrollment(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM enrollments WHERE user_id=$1 AND class_id=$2`, studentID, classID); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("start after leaving the class = %d, want 404", code)
	}
	assertNotStarted(t, pool, aid, studentID)
	var inbox struct {
		Items []any `json:"items"`
	}
	if code := getJSON(t, h, student, "/api/v1/lite/inbox", &inbox); code != http.StatusOK || inbox.Items == nil || len(inbox.Items) != 0 {
		t.Fatalf("inbox after leaving the class = %d %+v, want empty", code, inbox)
	}
}

func assertNotStarted(t *testing.T, pool *pgxpool.Pool, aid string, studentID uuid.UUID) {
	t.Helper()
	var started bool
	if err := pool.QueryRow(context.Background(),
		`SELECT started_at IS NOT NULL OR atom_id IS NOT NULL FROM lite_assignment_recipient WHERE assignment_id=$1 AND user_id=$2`,
		aid, studentID).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if started {
		t.Fatal("recipient marked started after a failed start")
	}
}

func TestStartOtherStudentAndArchived404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	other := createStudent(t, pool, SeedSchoolID, "as-not-recipient@demo.local")
	enrollStudent(t, pool, other, classID)
	if code := assignJSON(t, h, signInAs(t, pool, other), "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("non-recipient start = %d", code)
	}
	assignJSON(t, h, teacher, "DELETE", "/api/v1/lite/teacher/assignments/"+aid, nil, nil)
	student := signInAs(t, pool, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("archived start = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/seen", nil, nil); code != http.StatusNotFound {
		t.Fatalf("archived seen = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/not-a-uuid/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("malformed id start = %d", code)
	}
	var inbox struct {
		Items []any `json:"items"`
	}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Items == nil || len(inbox.Items) != 0 {
		t.Fatalf("archived still in inbox (or items not an array): %+v", inbox)
	}
	assertNotStarted(t, pool, aid, studentID)
}

func TestForAtomReturnsAssignment(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	started := startAssignment(t, h, student, aid)
	type forAtomResp struct {
		Assignment *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			DueAt string `json:"dueAt"`
		} `json:"assignment"`
	}
	var out forAtomResp
	if code := getJSON(t, h, student, "/api/v1/lite/assignments/for-atom/"+started.AtomID, &out); code != http.StatusOK {
		t.Fatalf("for-atom = %d", code)
	}
	if out.Assignment == nil || out.Assignment.ID != aid || out.Assignment.Title != "雨" || out.Assignment.DueAt == "" {
		t.Fatalf("for-atom = %+v", out)
	}
	// Another student's atom id → assignment null (not 404, not leaked).
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "as-for-atom-other@demo.local"))
	out = forAtomResp{}
	if code := getJSON(t, h, other, "/api/v1/lite/assignments/for-atom/"+started.AtomID, &out); code != http.StatusOK {
		t.Fatalf("other for-atom = %d, want 200", code)
	}
	if out.Assignment != nil {
		t.Fatal("for-atom leaked another student's assignment")
	}
	// An atom she made herself, outside any assignment → null.
	own := seedLiteWritingForUser(t, pool, studentID, "active")
	out = forAtomResp{}
	getJSON(t, h, student, "/api/v1/lite/assignments/for-atom/"+own.String(), &out)
	if out.Assignment != nil {
		t.Fatalf("for-atom on an unassigned atom = %+v", out)
	}
}

// TestAssignedWritingFinishStatus: finishing the started writing is what the
// teacher sees as done, or done_late when the deadline had already passed.
func TestAssignedWritingFinishStatus(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	ctx := context.Background()

	finish := func(aid string) string {
		t.Helper()
		started := startAssignment(t, h, student, aid)
		if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+started.AtomID+"/draft", map[string]any{"body": "雨下了一整天。"}, nil); code != http.StatusOK {
			t.Fatalf("put draft = %d", code)
		}
		if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+started.AtomID+"/finish", nil, nil); code != http.StatusOK {
			t.Fatalf("finish = %d", code)
		}
		var detail struct {
			Recipients []struct {
				Status string `json:"status"`
			} `json:"recipients"`
		}
		getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail)
		if len(detail.Recipients) != 1 {
			t.Fatalf("recipients = %+v", detail.Recipients)
		}
		return detail.Recipients[0].Status
	}

	onTime := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	if got := finish(onTime); got != "done" {
		t.Fatalf("on-time finish status = %q, want done", got)
	}

	late := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	if _, err := pool.Exec(ctx, `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id=$1`, late); err != nil {
		t.Fatal(err)
	}
	if got := finish(late); got != "done_late" {
		t.Fatalf("late finish status = %q, want done_late", got)
	}
}
