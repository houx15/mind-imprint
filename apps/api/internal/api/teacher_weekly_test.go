package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
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
