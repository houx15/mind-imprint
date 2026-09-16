package api_test

// lite_class_summary_test.go — D2's one-sentence class digest. What these
// tests hold down: who may call the route, that §6 grounding actually fails
// a bad reply (and never caches it), that a same-day-same-roster second call
// is free, and that every real model call leaves a billable llm_call row on
// the digest class.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// liteClassSummaryFixture is liteTeacherFixtureWithProvider plus a Route that
// insists the call is on gateway.ClassDigest — dialogue would be money spent
// on the wrong tier for a "one line, off the page she is already looking
// at" summary — and stamps the resolved tier "digest" so the metering
// assertions can read it straight off the row.
func liteClassSummaryFixture(t *testing.T, prov gateway.Provider) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	pool = newAPITestPool(t)
	route := func(class string) gateway.KeyResolver {
		return func(context.Context) (gateway.Resolved, error) {
			if class != gateway.ClassDigest {
				return gateway.Resolved{}, fmt.Errorf("lite class summary routed class %q, want %q", class, gateway.ClassDigest)
			}
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: gateway.ClassDigest}, nil
		}
	}
	h = New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: prov, Route: route, SpecByID: cards.ByID,
	}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacher = signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "cs-teacher@demo.local"))
	classID = createClassViaAPI(t, h, teacher, "Lite Summary Class")
	studentID = createStudent(t, pool, SeedSchoolID, "cs-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	return
}

func summaryPath(classID string) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/summary"
}

type classSummaryJSON struct {
	Summary     string `json:"summary"`
	GeneratedAt string `json:"generatedAt"`
	Cached      bool   `json:"cached"`
}

func postSummary(t *testing.T, h http.Handler, c *http.Cookie, classID string) (int, classSummaryJSON, string) {
	t.Helper()
	rec := doJSON(t, h, c, "POST", summaryPath(classID), "")
	var out classSummaryJSON
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode summary: %v body=%s", err, rec.Body)
		}
	}
	return rec.Code, out, rec.Body.String()
}

// TestClassSummaryRejectsOutsiders — 401 signed out, 403 a student, 404 a
// teacher who does not teach this class (the same not-found every other lite
// teacher route gives, so the class id cannot be probed).
func TestClassSummaryRejectsOutsiders(t *testing.T) {
	h, pool, _, classID, studentID := liteClassSummaryFixture(t, gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳。")))

	// doJSON's withCookie panics on a nil cookie (net/http.Request.AddCookie
	// does not accept one), so the signed-out case builds the request by hand.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", summaryPath(classID), nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out = %d, want 401; body=%s", rec.Code, rec.Body)
	}

	student := signInAs(t, pool, studentID)
	if rec := doJSON(t, h, student, "POST", summaryPath(classID), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403; body=%s", rec.Code, rec.Body)
	}

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "cs-other@demo.local"))
	if rec := doJSON(t, h, other, "POST", summaryPath(classID), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestClassSummaryStubReturns200 — a well-behaved reply reaches the teacher,
// unmodified, with generatedAt set and cached false on the first call.
func TestClassSummaryStubReturns200(t *testing.T) {
	h, pool, teacher, classID, studentID := liteClassSummaryFixture(t, gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。")))
	renameLiteStudent(t, pool, studentID, "林知遥")

	code, out, body := postSummary(t, h, teacher, classID)
	if code != http.StatusOK {
		t.Fatalf("summary = %d, want 200; body=%s", code, body)
	}
	if out.Summary == "" {
		t.Fatalf("empty summary; body=%s", body)
	}
	if out.GeneratedAt == "" {
		t.Fatalf("empty generatedAt; body=%s", body)
	}
	if out.Cached {
		t.Fatalf("first call reported cached=true; body=%s", body)
	}
}

// TestClassSummaryCachedSameDaySameData — a second request for the same
// class, same day, same roster, gets cached=true and the stub is not called
// again.
func TestClassSummaryCachedSameDaySameData(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。"))
	h, _, teacher, classID, _ := liteClassSummaryFixture(t, prov)

	code1, out1, body1 := postSummary(t, h, teacher, classID)
	if code1 != http.StatusOK {
		t.Fatalf("first call = %d, want 200; body=%s", code1, body1)
	}
	if out1.Cached {
		t.Fatalf("first call reported cached=true; body=%s", body1)
	}

	code2, out2, body2 := postSummary(t, h, teacher, classID)
	if code2 != http.StatusOK {
		t.Fatalf("second call = %d, want 200; body=%s", code2, body2)
	}
	if !out2.Cached {
		t.Fatalf("second call (same day, same roster) reported cached=false; body=%s", body2)
	}
	if out2.Summary != out1.Summary {
		t.Fatalf("cached summary differs from the original: %q vs %q", out2.Summary, out1.Summary)
	}
	if prov.Calls != 1 {
		t.Fatalf("model called %d times, want 1 (second request should have been served from cache)", prov.Calls)
	}
}

// TestClassSummaryRosterChangeRecomputes — a roster row's activity fields
// changing between the two requests changes the cache key, so the second
// request is a fresh computation (and calls the model again).
func TestClassSummaryRosterChangeRecomputes(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。"),
		weeklyReply("这周有学生完成了写作练习。"),
	)
	h, pool, teacher, classID, studentID := liteClassSummaryFixture(t, prov)

	code1, _, body1 := postSummary(t, h, teacher, classID)
	if code1 != http.StatusOK {
		t.Fatalf("first call = %d, want 200; body=%s", code1, body1)
	}
	if prov.Calls != 1 {
		t.Fatalf("model called %d times after the first request, want 1", prov.Calls)
	}

	// Change the roster's activity fields: one finished writing bumps
	// WritingsDone, which is part of the fingerprint.
	seedLiteWritingForUser(t, pool, studentID, "finished")

	code2, out2, body2 := postSummary(t, h, teacher, classID)
	if code2 != http.StatusOK {
		t.Fatalf("second call = %d, want 200; body=%s", code2, body2)
	}
	if out2.Cached {
		t.Fatalf("roster change served a stale cached summary; body=%s", body2)
	}
	if prov.Calls != 2 {
		t.Fatalf("model called %d times after the roster changed, want 2", prov.Calls)
	}
}

// TestClassSummaryUngroundedNameFails — the model names a real classmate who
// was never handed to it as evidence (no praise/watch card that week): 502,
// and the failure is not cached — a following call tries the model again.
func TestClassSummaryUngroundedNameFails(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		weeklyReply("建议老师联系林知遥，了解她这周的学习情况。"),
		weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。"),
	)
	h, pool, teacher, classID, _ := liteClassSummaryFixture(t, prov)

	// The fixture's own student gets the default "never_used" watch card
	// (zero activity in the last completed week) — grounded. second gets
	// exactly one active day in that same week and nothing else, so Cards()
	// returns nil, nil for her: she has NO card, but she IS a real roster
	// student.
	second := createStudent(t, pool, SeedSchoolID, "cs-uncarded@demo.local")
	enrollStudent(t, pool, second, classID)
	renameLiteStudent(t, pool, second, "林知遥")

	ws, _ := weeklyWindow()
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "reading", UserID: second})
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	mustExec(t, pool, `INSERT INTO atom_active_day (atom_id, day, seconds) VALUES ($1, $2, 60)`,
		atom.ID, ws.AddDate(0, 0, 1))

	code, _, body := postSummary(t, h, teacher, classID)
	if code != http.StatusBadGateway {
		t.Fatalf("summary with an ungrounded name = %d, want 502; body=%s", code, body)
	}
	if !strings.Contains(body, "摘要生成失败") {
		t.Fatalf("error body does not read 「摘要生成失败」: %s", body)
	}

	// Not cached: the next call re-runs the model (a well-behaved reply this
	// time) and succeeds.
	code2, out2, body2 := postSummary(t, h, teacher, classID)
	if code2 != http.StatusOK {
		t.Fatalf("retry after a rejected summary = %d, want 200; body=%s", code2, body2)
	}
	if out2.Cached {
		t.Fatalf("retry after a rejected summary reported cached=true; body=%s", body2)
	}
	if prov.Calls != 2 {
		t.Fatalf("model called %d times, want 2 (the failed attempt must not have been cached)", prov.Calls)
	}
}

// TestClassSummaryUngroundedCountFails — the model states a head count ("N
// 位/名/人") that none of the statistics handed to it support: 502.
func TestClassSummaryUngroundedCountFails(t *testing.T) {
	h, _, teacher, classID, _ := liteClassSummaryFixture(t, gateway.NewStubProvider(weeklyReply("这周有 99 位学生完成了写作练习。")))

	code, _, body := postSummary(t, h, teacher, classID)
	if code != http.StatusBadGateway {
		t.Fatalf("summary with an ungrounded count = %d, want 502; body=%s", code, body)
	}
	if !strings.Contains(body, "摘要生成失败") {
		t.Fatalf("error body does not read 「摘要生成失败」: %s", body)
	}
}

// TestClassSummaryMetersOnDigest — every real call leaves one llm_call row,
// on gateway.ClassDigest (the fixture's Route errors on any other class).
func TestClassSummaryMetersOnDigest(t *testing.T) {
	h, pool, teacher, classID, _ := liteClassSummaryFixture(t, gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。")))

	if code, _, body := postSummary(t, h, teacher, classID); code != http.StatusOK {
		t.Fatalf("summary = %d, want 200; body=%s", code, body)
	}

	var purpose, model, tier, surface string
	var prompt, completion int
	err := pool.QueryRow(context.Background(),
		`SELECT purpose, model, tier, surface, prompt_tokens, completion_tokens FROM llm_call ORDER BY created_at DESC LIMIT 1`,
	).Scan(&purpose, &model, &tier, &surface, &prompt, &completion)
	if err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if purpose != "lite_class_summary" || surface != "lite" {
		t.Fatalf("llm_call purpose=%q surface=%q", purpose, surface)
	}
	if tier != gateway.ClassDigest {
		t.Fatalf("llm_call tier=%q, want %q (the fixture's Route only answers ClassDigest)", tier, gateway.ClassDigest)
	}
	if prompt != 100 || completion != 50 {
		t.Fatalf("llm_call tokens = %d/%d, want the usage the call reported", prompt, completion)
	}
}
