package api_test

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

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func TestWeeklyReportRejectsForeignTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")

	intruder := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-intruder@demo.local"))
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), intruder)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404 — a foreign class must be existence-hidden", rec.Code)
	}
}

func TestWeeklyReportRejectsStudent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner2@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")

	student := signInSeed(t, pool)
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), student)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", rec.Code)
	}
}

func TestWeeklyReportShapeWithoutProse(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-shape@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")
	studentID := createStudent(t, pool, SeedSchoolID, "wk-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), owner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var got WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Stats) != 4 {
		t.Fatalf("stats = %d; want 4", len(got.Stats))
	}
	if got.ProseReady {
		t.Fatal("proseReady = true; nothing has been generated yet")
	}
	if got.Comment != nil {
		t.Fatalf("comment = %v; want null before generation", *got.Comment)
	}
	if got.ClassSize != 1 {
		t.Fatalf("classSize = %d; want 1", got.ClassSize)
	}
	if got.WeekLabel == "" {
		t.Fatal("weekLabel must always be present")
	}
}

// weeklyProseReplyFor builds a stub composer reply that covers exactly the
// cards the rule layer will have produced. ComposeWeekly rejects a reply whose
// card set does not match the fact sheet — a fixed `"cards":[]` string would be
// rejected the moment the class contains a flagged student, which is precisely
// the situation these tests set up.
func weeklyProseReplyFor(userIDs ...string) string {
	cards := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		cards = append(cards, fmt.Sprintf(`{"userId":%q,"lead":"连续让 AI 直接给结论","action":"线下问一句这些数据凭什么说明影响。"}`, id))
	}
	return fmt.Sprintf(`{"comment":"这周整体在往会自己想挪。","depthNote":"熟练档多了一人。","autonomyNote":"自主均分小幅上行。","cards":[%s]}`,
		strings.Join(cards, ","))
}

// sequenceStubProvider is a gateway.Provider that replays a different scripted
// reply on each successive Stream call, in order. assessStubProvider (and its
// composeStub twin in internal/agent) can only script ONE fixed reply for the
// whole test — but ComposeWeekly rejects any reply whose card set doesn't
// match the fact sheet it was given, so a top-up test (whose two calls carry
// two different fact sheets — the original student, then only the newly
// appeared one) cannot be served by a single fixed reply. Each call delegates
// to a fresh gateway.NewStubProvider so the event shape stays identical to
// assessStubProvider; calls past the end of the script repeat the last reply
// rather than panicking, so a stray extra call surfaces as a validation
// rejection (a wrong-shaped reply for that fact sheet) rather than a crash.
type sequenceStubProvider struct {
	replies []string
	calls   int
}

func newSequenceStubProvider(replies ...string) *sequenceStubProvider {
	return &sequenceStubProvider{replies: replies}
}

func (s *sequenceStubProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	i := s.calls
	if i >= len(s.replies) {
		i = len(s.replies) - 1
	}
	s.calls++
	reply := s.replies[i]
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}).Stream(ctx, r, req)
}

// TestWeeklyProseTopsUpACardThatAppearedAfterGeneration covers DEC-6: a card
// that fires only after the row already exists must be composed for and
// APPENDED, without ever rewriting anything already written. The first POST
// generates prose for one flagged student; a second student is then enrolled
// (also zero-activity, so she fires never_used); the second POST must compose
// wording for ONLY her — not resend the whole class — and append it, leaving
// the original comment and the first student's wording untouched. A third
// POST, with nothing new, must not spend again.
func TestWeeklyProseTopsUpACardThatAppearedAfterGeneration(t *testing.T) {
	pool := newAPITestPool(t)
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-topup@demo.local"))
	studentA := createStudent(t, pool, SeedSchoolID, "wk-topup-a@demo.local")
	studentB := createStudent(t, pool, SeedSchoolID, "wk-topup-b@demo.local")

	deps := DepsForTest(pool)
	deps.Provider = newSequenceStubProvider(
		weeklyProseReplyFor(studentA.String()),
		weeklyProseReplyFor(studentB.String()),
	)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()
	classID := createClassViaAPI(t, h, owner, "周报班")
	enrollStudent(t, pool, studentA, classID)

	post := func() WeeklyReportDTO {
		req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		var dto WeeklyReportDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return dto
	}
	findCard := func(dto WeeklyReportDTO, userID string) *WeeklyCardDTO {
		for _, list := range [][]WeeklyCardDTO{dto.Praise, dto.Watch} {
			for i := range list {
				if list[i].UserID == userID {
					return &list[i]
				}
			}
		}
		return nil
	}

	// 1. First POST creates the row with wording for student A only.
	first := post()
	if !first.ProseReady || first.Comment == nil {
		t.Fatalf("first POST did not produce prose: %+v", first)
	}
	firstComment := *first.Comment
	cardA := findCard(first, studentA.String())
	if cardA == nil || cardA.Lead == "" || cardA.Action == "" {
		t.Fatalf("student A must have wording after the first POST: %+v", first)
	}

	// A new student appears and fires never_used (zero activity).
	enrollStudent(t, pool, studentB, classID)

	// 2. Second POST composes wording for ONLY the new student and appends it.
	second := post()
	if second.Comment == nil {
		t.Fatal("second POST must still return a comment")
	}
	// 3. The original comment is unchanged by the top-up.
	if *second.Comment != firstComment {
		t.Fatalf("comment changed by top-up: got %q, want unchanged %q", *second.Comment, firstComment)
	}
	// 4. Both the original card's wording and the new card's wording are present.
	cardA2 := findCard(second, studentA.String())
	if cardA2 == nil || cardA2.Lead != cardA.Lead || cardA2.Action != cardA.Action {
		t.Fatalf("student A's original wording must survive the top-up: before %+v, after %+v", cardA, cardA2)
	}
	cardB := findCard(second, studentB.String())
	if cardB == nil || cardB.Lead == "" || cardB.Action == "" {
		t.Fatalf("student B must have wording after the top-up: %+v", second)
	}

	// Cross-check directly against the stored jsonb: exactly 2 cards, not a
	// resend of the whole class re-wrapped as one array.
	var cardsJSON []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT cards FROM class_weekly_prose WHERE class_id = $1`, classID).Scan(&cardsJSON); err != nil {
		t.Fatalf("query stored cards: %v", err)
	}
	var stored []struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(cardsJSON, &stored); err != nil {
		t.Fatalf("decode stored cards: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored cards = %d; want exactly 2 (one appended, not resent)", len(stored))
	}

	// 5. Exactly TWO llm_call rows — one genuine composition per POST that
	// actually had something new to say.
	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 2 {
		t.Fatalf("llm_call rows = %d; want exactly 2 — one per genuine composition", calls)
	}

	// 6. A third POST, class unchanged, makes no further call and returns the
	// same comment — the row is complete, nothing recomposes.
	third := post()
	if third.Comment == nil || *third.Comment != firstComment {
		t.Fatalf("third POST comment = %v; want unchanged %q", third.Comment, firstComment)
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call after third POST: %v", err)
	}
	if calls != 2 {
		t.Fatalf("llm_call rows after third POST = %d; want still 2 — a complete row must not recompose", calls)
	}
}

func TestWeeklyProseGeneratesOnceAndIsReadOnlyAfterwards(t *testing.T) {
	pool := newAPITestPool(t)
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-prose@demo.local"))
	// A student with zero activity fires never_used, so the fact sheet is
	// non-empty and the composer actually runs. Without this the handler's
	// "nothing to say" branch short-circuits and the test asserts nothing.
	studentID := createStudent(t, pool, SeedSchoolID, "wk-prose-s@demo.local")

	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(weeklyProseReplyFor(studentID.String()))
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()
	classID := createClassViaAPI(t, h, owner, "周报班")
	enrollStudent(t, pool, studentID, classID)

	post := func() WeeklyReportDTO {
		req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		var dto WeeklyReportDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return dto
	}

	first := post()
	if !first.ProseReady || first.Comment == nil {
		t.Fatalf("first POST did not produce prose: %+v", first)
	}
	second := post()
	if second.Comment == nil || *second.Comment != *first.Comment {
		t.Fatal("second POST must return the first comment — first-open-wins")
	}

	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("llm_call rows = %d; want exactly 1 — the second POST must not spend", calls)
	}
}

func TestWeeklyProseFailureStillReturnsTheScreen(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(`not json at all`)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-fail@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")
	enrollStudent(t, pool, createStudent(t, pool, SeedSchoolID, "wk-fail-s@demo.local"), classID)

	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; a failed composition must never wall the screen", rec.Code)
	}
	var dto WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ProseReady || dto.Comment != nil {
		t.Fatalf("prose must be absent after a rejected composition: %+v", dto)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("stats = %d; the numbers must still render", len(dto.Stats))
	}
	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("llm_call rows = %d; want 1 — cost is recorded even on rejection", calls)
	}
}

func TestWeeklyProseMakesNoCallForAnEmptyClass(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(weeklyProseReplyFor())
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-empty@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班") // no students enrolled

	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 0 {
		t.Fatalf("llm_call rows = %d; an empty class has nothing to say about", calls)
	}
}

// TestWeeklyReportForSeededClass exercises the GET path over migration 0034's
// seeded week (吴老师's IBDP 一年级 · 研究组, class ...0902 from 0029) rather
// than a hand-built fixture — the class a fresh dev DB actually renders. It
// asserts what 0034 promises: 罗一 (...0919, zero events) fires never_used
// unconditionally, every card carries non-empty evidence, and 吴桐's two
// seeded reports (L2 → L3 max depth) give the depth distribution a rated
// student.
//
// 陈屿's dropped_off card (...0914) is NOT asserted unconditionally. The rule
// is `ActiveDays <= PrevActiveDays - 2`, and teacher.PrevWindow deliberately
// compares against only [prevStart, prevStart+elapsed) — the SAME elapsed
// offset into last week that `now` sits at in this one — not a full previous
// week. 0034 seeds 陈屿's five previous-week events at Mon–Fri 00:05 UTC (the
// earliest honest placement: distinct calendar dates, right at each day's
// start), so PrevActiveDays only reaches 3 (enough to satisfy the rule against
// this week's ActiveDays=1) once elapsed exceeds 2 days + 5 minutes — i.e.
// from Wednesday 00:05 UTC of the current week onward. Before that moment, no
// seed can make dropped_off fire; that is PrevWindow's correct behaviour, not
// a seeding gap. So this test computes reachability from the response's own
// weekStart/asOf (the exact inputs PrevWindow was built from) and asserts the
// card's presence in the reachable regime, and its ABSENCE in the unreachable
// one — pinning both regimes instead of merely skipping one. The rule's own
// firing logic already has deterministic unit coverage in
// internal/teacher/weekly_test.go; this test's job is the seed, not the rule.
func TestWeeklyReportForSeededClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	wu := signInAs(t, pool, uuid.MustParse("00000000-0000-0000-0000-000000000910"))

	req := withCookie(httptest.NewRequest(http.MethodGet,
		"/api/v1/classes/00000000-0000-0000-0000-000000000902/weekly-report", nil), wu)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var dto WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ClassSize != 9 {
		t.Fatalf("classSize = %d; want 9", dto.ClassSize)
	}
	byTag := map[string]bool{}
	for _, c := range append(append([]WeeklyCardDTO{}, dto.Watch...), dto.Praise...) {
		byTag[c.TagCode] = true
		if c.Evidence == "" {
			t.Fatalf("card %s has no evidence — 每个判断带证据", c.TagCode)
		}
	}
	if !byTag["never_used"] {
		t.Fatalf("seeded class produced no never_used card; tags = %v", byTag)
	}

	weekStart, err := time.Parse(time.RFC3339, dto.WeekStart)
	if err != nil {
		t.Fatalf("parse weekStart %q: %v", dto.WeekStart, err)
	}
	asOf, err := time.Parse(time.RFC3339, dto.AsOf)
	if err != nil {
		t.Fatalf("parse asOf %q: %v", dto.AsOf, err)
	}
	elapsed := asOf.Sub(weekStart)
	// The threshold matches 0034's own arithmetic: PrevActiveDays only picks
	// up the 3rd of 陈屿's five Mon–Fri 00:05 UTC events (Wednesday's) once
	// PrevWindow's upper bound — prevStart + elapsed — passes that event's
	// timestamp, i.e. once elapsed strictly exceeds 2 days + 5 minutes.
	reachable := elapsed > 2*24*time.Hour+5*time.Minute
	if reachable {
		if !byTag["dropped_off"] {
			t.Fatalf("elapsed %s past week start (reachable regime) but no dropped_off card; tags = %v", elapsed, byTag)
		}
	} else {
		if byTag["dropped_off"] {
			t.Fatalf("elapsed %s past week start (unreachable regime — before Wed 00:05 UTC) but dropped_off card present; tags = %v", elapsed, byTag)
		}
	}

	if dto.Depth.RatedCount == 0 {
		t.Fatal("ratedCount = 0; the seeded class has evaluations")
	}
}

func TestWeeklyProseRejectsStudent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-guard@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")
	student := signInSeed(t, pool)
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), student)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", rec.Code)
	}
}
