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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweek"
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

// addUncardedStudent enrolls a second, real roster student who gets NO
// praise/watch card this (current, in-progress) week: exactly one active day
// today, no assignments, no keywords, no stalled items — Cards() returns
// nil, nil for her. She is exactly the case UngroundedNames exists to catch:
// a real classmate the model was never handed as evidence.
func addUncardedStudent(t *testing.T, pool *pgxpool.Pool, classID, email, name string) uuid.UUID {
	t.Helper()
	id := createStudent(t, pool, SeedSchoolID, email)
	enrollStudent(t, pool, id, classID)
	renameLiteStudent(t, pool, id, name)
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "reading", UserID: id})
	if err != nil {
		t.Fatalf("addUncardedStudent: create atom: %v", err)
	}
	mustExec(t, pool, `INSERT INTO atom_active_day (atom_id, day, seconds) VALUES ($1, $2, 60)`,
		atom.ID, liteweek.Day(time.Now()))
	return id
}

// lastUserPrompt reads the content of the last message a StubProvider
// received — the facts liteClassSummaryFacts built for that call.
func lastUserPrompt(t *testing.T, prov *gateway.StubProvider) string {
	t.Helper()
	msgs := prov.LastRequest.Messages
	if len(msgs) == 0 {
		t.Fatal("provider never received a request")
	}
	return msgs[len(msgs)-1].Content
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

// TestClassSummaryGroundingRetrySucceeds — a grounding failure
// gets ONE retry inside the same request, not an immediate 502. First reply
// names a real, uncarded classmate (ungrounded); the retry's reply is clean:
// the request succeeds with the SECOND reply, and both attempts are metered.
func TestClassSummaryGroundingRetrySucceeds(t *testing.T) {
	const cleanReply = "这周班级整体参与平稳，暂无需要特别关注的学生。"
	prov := gateway.NewSequenceStubProvider(
		weeklyReply("建议老师联系林知遥，了解她这周的学习情况。"),
		weeklyReply(cleanReply),
	)
	h, pool, teacher, classID, _ := liteClassSummaryFixture(t, prov)
	addUncardedStudent(t, pool, classID, "cs-uncarded@demo.local", "林知遥")

	code, out, body := postSummary(t, h, teacher, classID)
	if code != http.StatusOK {
		t.Fatalf("summary = %d, want 200 (the retry should have produced a clean reply); body=%s", code, body)
	}
	if out.Summary != cleanReply {
		t.Fatalf("summary = %q, want the retry's (second, grounded) reply %q", out.Summary, cleanReply)
	}
	if out.Cached {
		t.Fatalf("a freshly computed summary reported cached=true; body=%s", body)
	}
	if prov.Calls != 2 {
		t.Fatalf("model called %d times, want 2 (one rejected attempt + one retry)", prov.Calls)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM llm_call WHERE purpose = $1`, "lite_class_summary"); n != 2 {
		t.Fatalf("llm_call rows = %d, want 2 (both attempts must be metered)", n)
	}
}

// TestClassSummaryGroundingRetryBothFail — both the original attempt and the
// retry name the same uncarded classmate: 502, two metered calls, and the
// rejection is not cached — a following request re-runs the model rather than
// replaying a stored failure.
func TestClassSummaryGroundingRetryBothFail(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		weeklyReply("建议老师联系林知遥，了解她这周的学习情况。"),
		weeklyReply("请老师也关注一下林知遥这周的状态。"),
	)
	h, pool, teacher, classID, _ := liteClassSummaryFixture(t, prov)
	addUncardedStudent(t, pool, classID, "cs-uncarded@demo.local", "林知遥")

	code, _, body := postSummary(t, h, teacher, classID)
	if code != http.StatusBadGateway {
		t.Fatalf("summary with two ungrounded replies = %d, want 502; body=%s", code, body)
	}
	if !strings.Contains(body, "摘要生成失败") {
		t.Fatalf("error body does not read 「摘要生成失败」: %s", body)
	}
	if prov.Calls != 2 {
		t.Fatalf("model called %d times, want 2 (the retry must still have run)", prov.Calls)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM llm_call WHERE purpose = $1`, "lite_class_summary"); n != 2 {
		t.Fatalf("llm_call rows = %d, want 2", n)
	}

	// Not cached: a following request calls the model again.
	// SequenceStubProvider clamps to its last script once exhausted, so this
	// replays the same ungrounded reply and 502s again — the point is that
	// Calls keeps climbing, proving the rejected result was never stored.
	code2, _, body2 := postSummary(t, h, teacher, classID)
	if code2 != http.StatusBadGateway {
		t.Fatalf("second request = %d, want 502 again; body=%s", code2, body2)
	}
	if prov.Calls <= 2 {
		t.Fatalf("model called %d times after a second request, want more than 2 (the rejected result must not have been cached)", prov.Calls)
	}
}

// TestClassSummaryAssignmentRateNotGroundingEvidence — the
// completion RATE must never ground a head count. Measured before the fix:
// grounded counts [30,10,5,42] (42 = AssignmentRate) let 「本周有 42 位学生
// 完成了写作练习」 through — 42 was a percentage, not a student count.
// Reproduced with a rate of 100 (one assignment due, one done on time, class
// size 1): nothing but the (now-excluded) rate would ever have grounded a
// "100 位学生" claim in a one-student class.
func TestClassSummaryAssignmentRateNotGroundingEvidence(t *testing.T) {
	h, pool, teacher, classID, studentID := liteClassSummaryFixture(t,
		gateway.NewStubProvider(weeklyReply("本周有 100 位学生完成了写作练习。")))
	student := signInAs(t, pool, studentID)
	ws := liteweek.WeekStart(time.Now())

	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	started := startAssignment(t, h, student, aid)
	mustExec(t, pool, `UPDATE lite_assignment SET due_at = $2 WHERE id = $1`, aid, ws.AddDate(0, 0, 5))
	mustExec(t, pool, `UPDATE writing SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, started.AtomID, time.Now())

	code, _, body := postSummary(t, h, teacher, classID)
	if code != http.StatusBadGateway {
		t.Fatalf("summary laundering the assignment rate as a head count = %d, want 502; body=%s", code, body)
	}
	if !strings.Contains(body, "摘要生成失败") {
		t.Fatalf("error body does not read 「摘要生成失败」: %s", body)
	}
}

// TestClassSummaryReadsCurrentWeek — the summary must read the
// SAME week the card above it already shows (§12.5, "和卡片用同一批数据，不
// 另取") — the current, in-progress Beijing week, not the last completed one.
// Pinned by inspecting the actual prompt sent to the model
// (StubProvider.LastRequest): an item finished only in the last completed
// week must not count toward this week's facts; an item finished today must.
func TestClassSummaryReadsCurrentWeek(t *testing.T) {
	// Case A: finished only last week.
	provA := gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。"))
	hA, poolA, teacherA, classIDA, studentIDA := liteClassSummaryFixture(t, provA)
	lastWs, _ := weeklyWindow()
	rA := seedOldReading(t, poolA, studentIDA, "上周读完", lastWs)
	mustExec(t, poolA, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, rA, lastWs.AddDate(0, 0, 2))

	if code, _, body := postSummary(t, hA, teacherA, classIDA); code != http.StatusOK {
		t.Fatalf("summary = %d, want 200; body=%s", code, body)
	}
	if promptA := lastUserPrompt(t, provA); strings.Contains(promptA, "完成项数：1") {
		t.Fatalf("an item finished only last week counted toward this week's facts:\n%s", promptA)
	}

	// Case B: finished today (this, in-progress week).
	provB := gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。"))
	hB, poolB, teacherB, classIDB, studentIDB := liteClassSummaryFixture(t, provB)
	rB := seedOldReading(t, poolB, studentIDB, "这周读完", liteweek.WeekStart(time.Now()))
	mustExec(t, poolB, `UPDATE reading SET status = 'finished', finished_at = $2 WHERE atom_id = $1`, rB, time.Now())

	if code, _, body := postSummary(t, hB, teacherB, classIDB); code != http.StatusOK {
		t.Fatalf("summary = %d, want 200; body=%s", code, body)
	}
	if promptB := lastUserPrompt(t, provB); !strings.Contains(promptB, "完成项数：1") {
		t.Fatalf("an item finished this week did not appear in the facts:\n%s", promptB)
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

// TestClassSummaryComputationIgnoresTheFirstRequesterCancelling — the
// summary is computed once per key and every concurrent request for that key
// waits on the result. The computation runs on the first request's
// goroutine, so if it read that request's context, a teacher who navigated
// away would fail the summary for everyone waiting on it. A request whose
// context is already cancelled must still produce the summary.
func TestClassSummaryComputationIgnoresTheFirstRequesterCancelling(t *testing.T) {
	pool := newAPITestPool(t)
	route := func(string) gateway.KeyResolver {
		return func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: gateway.ClassDigest}, nil
		}
	}
	a := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Route: route, SpecByID: cards.ByID,
		Provider: gateway.NewStubProvider(weeklyReply("这周班级整体参与平稳，暂无需要特别关注的学生。")),
	})
	h := a.Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacherID := createTeacher(t, pool, SeedSchoolID, "cs-cancel-teacher@demo.local")
	classID := createClassViaAPI(t, h, signInAs(t, pool, teacherID), "Lite Summary Cancel Class")
	enrollStudent(t, pool, createStudent(t, pool, SeedSchoolID, "cs-cancel-student@demo.local"), classID)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", summaryPath(classID), nil).WithContext(ctx)
	summary, err := a.ComposeLiteClassSummaryForTest(r, teacherID, uuid.MustParse(classID))
	if err != nil {
		t.Fatalf("summary with a cancelled first requester failed: %v", err)
	}
	if summary == "" {
		t.Fatal("empty summary")
	}
}

// TestClassSummaryListSizeIsGrounded — the watch list names two students, so
// 「两位学生需要关注」 states a count the facts gave. The live run of
// 2026-09-17 failed this sentence in 2 of 3 summaries. A count the lists do
// not support still fails, and each listed student carries her pronoun.
func TestClassSummaryListSizeIsGrounded(t *testing.T) {
	for _, tc := range []struct {
		reply string
		want  int
	}{
		{"本周有两位学生需要关注。", http.StatusOK},
		{"本周有 7 位学生需要关注。", http.StatusBadGateway},
	} {
		prov := gateway.NewStubProvider(weeklyReply(tc.reply))
		h, pool, teacher, classID, _ := liteClassSummaryFixture(t, prov)
		quiet := createStudent(t, pool, SeedSchoolID, "cs-quiet@demo.local")
		enrollStudent(t, pool, quiet, classID)
		addUncardedStudent(t, pool, classID, "cs-active@demo.local", "赵一诺")

		code, _, body := postSummary(t, h, teacher, classID)
		if code != tc.want {
			t.Fatalf("%q: summary = %d, want %d; body=%s", tc.reply, code, tc.want, body)
		}
		if prompt := prov.LastRequest.Messages[1].Content; !strings.Contains(prompt, "（称谓：未设置）") {
			t.Fatalf("facts lack the pronoun:\n%s", prompt)
		}
	}
}
