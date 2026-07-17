package api_test

// course_session_test.go — Task 7 (Slice 12): handler tests for the Course
// session surface (get-or-create, ownership, the SSE ask/advance turns, the
// phase floor's zero-LLM-call guarantee, thin card submit). Mirrors
// chat_test.go's testcontainers-backed pattern: real Postgres + a seeded
// student, a scripted JSON-emitting provider standing in for the model.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// courseSeededCourseID is the one course seeded by migration 0012, wired to
// the one course skill (info-literacy-course) seeded alongside it.
const courseSeededCourseID = "00000000-0000-0000-0000-0000000000c1"

// courseProvider is a gateway.Provider whose Stream emits jsonBody as the
// coach's ENTIRE reply text — ProposeCourseReply parses the model's output as
// a JSON object, so (unlike fakeProvider in studioturn_test.go, which emits
// plain prose) this must be valid `{"type":"reply",...}` / `{"type":"advance",...}`.
func courseProvider(jsonBody string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonBody},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// startSession POSTs the get-or-create session endpoint and decodes the DTO.
func startSession(t *testing.T, h http.Handler, cookie *http.Cookie, courseID string) CourseSessionDTO {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseID+"/session", nil), cookie))
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("start session: %d — %s", rr.Code, rr.Body.String())
	}
	var dto CourseSessionDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return dto
}

// TestCourseSession_CreateIsIdempotent — POST twice returns the same session
// id (the UNIQUE(user_id, course_id) upsert), and GET reflects the seeded
// skill's first phase.
func TestCourseSession_CreateIsIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	first := startSession(t, h, cookie, courseSeededCourseID)
	if first.ID == "" {
		t.Fatal("start session must return an id")
	}
	second := startSession(t, h, cookie, courseSeededCourseID)
	if second.ID != first.ID {
		t.Fatalf("second start minted a new session %s, want the existing %s", second.ID, first.ID)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("get session: %d — %s", rr2.Code, rr2.Body.String())
	}
	var got CourseSessionDTO
	if err := json.Unmarshal(rr2.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got.Phase != "demonstrate" {
		t.Fatalf("phase = %q, want demonstrate", got.Phase)
	}
	if got.PhaseTitle == "" {
		t.Fatal("phaseTitle must be resolved from the skill's Contract.Title")
	}
}

// TestCourseSession_OwnershipIs404 — a session belonging to a different
// student never surfaces to the caller: GET/ask/advance/submit/skip all 404
// (never 403) when the caller has no session of their own for this course,
// even though ANOTHER student's session (and its card instance) exists for
// the same course id. POST /session (start) is get-or-create FOR THE CALLER
// — it cannot ownership-fail by construction (UNIQUE is (user_id,course_id),
// not course_id alone), so it is verified separately: it must mint the
// caller's OWN session, distinct from the other student's.
func TestCourseSession_OwnershipIs404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"reply","body":"继续吧。"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	q := sqlc.New(pool)
	courseID := uuid.MustParse(courseSeededCourseID)
	other := createStudent(t, pool, SeedSchoolID, "course-other@demo.local")
	otherSess, err := q.CreateCourseSession(context.Background(), sqlc.CreateCourseSessionParams{
		UserID: other, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create other student's session: %v", err)
	}
	otherCard, err := q.CreateSessionCardInstance(context.Background(), sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(otherSess.ID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create other student's card instance: %v", err)
	}
	cookie := signInSeed(t, pool) // Phoebe (SeedUserID) — has no session of her own yet.

	cases := []struct {
		method, path, body string
	}{
		{"GET", "/api/v1/courses/" + courseSeededCourseID + "/session", ""},
		{"POST", "/api/v1/courses/" + courseSeededCourseID + "/session/ask", `{"user_input":"这条对吗"}`},
		{"POST", "/api/v1/courses/" + courseSeededCourseID + "/session/advance", ""},
		{"POST", "/api/v1/courses/" + courseSeededCourseID + "/session/cards/" + otherCard.ID.String() + "/submit",
			`{"field_values":{},"event_trace":[],"anchors":[]}`},
		{"POST", "/api/v1/courses/" + courseSeededCourseID + "/session/cards/" + otherCard.ID.String() + "/skip", ""},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		var req *http.Request
		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}
		h.ServeHTTP(rr, withCookie(req, cookie))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s %s: want 404, got %d — %s", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}

	// start is get-or-create FOR THE CALLER: it must succeed and mint
	// Phoebe's own session, never leaking or colliding with `other`'s.
	mine := startSession(t, h, cookie, courseSeededCourseID)
	if mine.ID == otherSess.ID.String() {
		t.Fatal("the caller's session must not be the other student's session")
	}
}

// TestCourseSession_Ask — a POST .../session/ask with a stub provider
// returning a reply streams a text frame then done, and records exactly one
// llm_call row (surface=course, purpose=coach, project_id NULL).
func TestCourseSession_Ask(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"reply","body":"你觉得这句话里，哪一部分是证据？"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/ask",
		strings.NewReader(`{"user_input":"这条是真的吗？"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("ask: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: text") {
		t.Fatalf("expected a text frame:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected a done frame:\n%s", body)
	}

	// 404 non-owned course id (never created a session there).
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/courses/00000000-0000-0000-0000-0000000009ff/session/ask",
		strings.NewReader(`{"user_input":"whatever"}`))
	h.ServeHTTP(rr2, withCookie(req2, cookie))
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("foreign course: want 404, got %d", rr2.Code)
	}

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'course' AND purpose = 'coach' AND project_id IS NULL`,
	).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("llm_call count = %d, want exactly 1", n)
	}
}

// TestCourseSession_AdvanceUnmetFloorSpendsNothing — no course_progress rows
// exist, so demonstrate's steps_viewed floor is unmet: advance must carry a
// text frame (what's owed), NEVER a phase frame, spend no llm_call, and leave
// the session's phase unchanged in the DB.
func TestCourseSession_AdvanceUnmetFloorSpendsNothing(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"advance","to":"guided"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("advance: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: text") {
		t.Fatalf("expected a text frame naming what's owed:\n%s", body)
	}
	if strings.Contains(body, "event: phase") {
		t.Fatalf("an unmet floor must never emit a phase frame:\n%s", body)
	}

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'course'`,
	).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 0 {
		t.Fatalf("llm_call count = %d, want 0 — an unmet floor must short-circuit BEFORE the model", n)
	}

	var phase string
	if err := pool.QueryRow(context.Background(),
		`SELECT phase FROM course_session WHERE user_id = $1 AND course_id = $2`, SeedUserID, courseSeededCourseID,
	).Scan(&phase); err != nil {
		t.Fatalf("query session phase: %v", err)
	}
	if phase != "demonstrate" {
		t.Fatalf("phase = %q, want unchanged demonstrate", phase)
	}
}

// TestCourseSession_AdvanceMovesThePhase — with course_progress reporting
// both demonstrate steps viewed, the floor is met, the coach's advance is
// accepted, and the SSE stream carries a phase frame; the DB row moves.
//
// Slice-12 whole-branch Critical-1 regression test: `steps_viewed` is earned
// by driving the REAL path a student uses — POSTing the per-ordinal render
// endpoint — rather than seeding course_progress.completed_ordinals directly
// via UpsertCourseProgress. Before the fix, NOTHING wrote completed_ordinals
// at that moment (CoursePlayer.tsx's `go()` only records the ordinal being
// LEFT, and never runs at all for the last ordinal of a phase, which routes
// through courseAdvance instead), so this exact sequence — render 0, render
// 1, then advance — reproduces the bug: without the render-handler fix, the
// floor would stay unmet forever and this test would fail at the phase
// frame / DB-phase assertions below.
func TestCourseSession_AdvanceMovesThePhase(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"advance","to":"guided"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	for _, ord := range []int{0, 1} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST",
			"/api/v1/courses/"+courseSeededCourseID+"/steps/"+strconv.Itoa(ord)+"/render", nil), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("render step %d: %d — %s", ord, rr.Code, rr.Body.String())
		}
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("advance: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: phase") || !strings.Contains(body, `"to":"guided"`) {
		t.Fatalf("expected a phase frame to guided:\n%s", body)
	}

	var phase string
	if err := pool.QueryRow(context.Background(),
		`SELECT phase FROM course_session WHERE user_id = $1 AND course_id = $2`, SeedUserID, courseSeededCourseID,
	).Scan(&phase); err != nil {
		t.Fatalf("query session phase: %v", err)
	}
	if phase != "guided" {
		t.Fatalf("phase = %q, want guided", phase)
	}
}

// TestCourseSession_ClientCannotAssertCompletedOrdinals is the Critical-3
// regression test: PUT /courses/{id}/progress carrying a spoofed
// completed_ordinals must NOT satisfy the steps_viewed floor — the client
// never rendered anything, it only asserted the array. Before the fix,
// putCourseProgress wrote the client's completed_ordinals verbatim via
// UpsertCourseProgress, so this exact PUT would have made the floor
// (falsely) met; after the fix it is silently ignored (current_ordinal is
// the only column PUT can write) and the advance is refused, zero llm_call.
func TestCourseSession_ClientCannotAssertCompletedOrdinals(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"advance","to":"guided"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	// The spoof: assert both demonstrate ordinals viewed without ever
	// rendering either.
	putBody := `{"current_ordinal":1,"completed_ordinals":[0,1]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/"+courseSeededCourseID+"/progress", strings.NewReader(putBody)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("put progress: %d — %s", rr.Code, rr.Body.String())
	}

	// The spoofed array must not have landed — the DB's completed_ordinals
	// stays empty, only current_ordinal took.
	var completed []int32
	var currentOrdinal int32
	if err := pool.QueryRow(context.Background(),
		`SELECT current_ordinal, completed_ordinals FROM course_progress WHERE user_id = $1 AND course_id = $2`,
		SeedUserID, courseSeededCourseID,
	).Scan(&currentOrdinal, &completed); err != nil {
		t.Fatalf("query course_progress: %v", err)
	}
	if currentOrdinal != 1 {
		t.Fatalf("current_ordinal = %d, want 1 (still a legitimate UX write)", currentOrdinal)
	}
	if len(completed) != 0 {
		t.Fatalf("completed_ordinals = %v, want empty — PUT /progress must never write the floor's input", completed)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("advance: %d — %s", rr2.Code, rr2.Body.String())
	}
	body := rr2.Body.String()
	if strings.Contains(body, "event: phase") {
		t.Fatalf("a client-spoofed completed_ordinals must NOT satisfy the floor:\n%s", body)
	}

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'course'`,
	).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 0 {
		t.Fatalf("llm_call count = %d, want 0 — the (still unmet) floor must short-circuit before the model", n)
	}
}

// TestCourseSession_CardSubmitIsThin — submitting a session card flips it to
// completed and writes NOTHING beyond the card row + its one event: no
// graph_node/graph_edge/intervention row appears that didn't already exist
// (the seeded demo project pre-populates some of these tables, so the
// assertion is a before/after delta, not an absolute zero).
func TestCourseSession_CardSubmitIsThin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	q := sqlc.New(pool)
	cookie := signInSeed(t, pool)
	sessDTO := startSession(t, h, cookie, courseSeededCourseID)
	sessID := uuid.MustParse(sessDTO.ID)

	ci, err := q.CreateSessionCardInstance(context.Background(), sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(sessID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}
	ci2, err := q.CreateSessionCardInstance(context.Background(), sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(sessID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance 2: %v", err)
	}

	count := func(q string) int {
		var n int
		if err := pool.QueryRow(context.Background(), q).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", q, err)
		}
		return n
	}
	nodesBefore := count(`SELECT count(*) FROM graph_node`)
	edgesBefore := count(`SELECT count(*) FROM graph_edge`)
	interventionsBefore := count(`SELECT count(*) FROM intervention`)

	// submit -> completed.
	rr := httptest.NewRecorder()
	submitBody := `{"field_values":{"q":"a"},"event_trace":[{"kind":"submit"}],"anchors":[]}`
	req := httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/cards/"+ci.ID.String()+"/submit", strings.NewReader(submitBody))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	var submitResp struct {
		CardStatus string `json:"card_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &submitResp); err != nil {
		t.Fatalf("decode submit resp: %v", err)
	}
	if submitResp.CardStatus != "completed" {
		t.Fatalf("card_status = %q, want completed", submitResp.CardStatus)
	}
	got, err := q.GetCardInstance(context.Background(), ci.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "completed" {
		t.Fatalf("persisted status = %q, want completed", got.Status)
	}

	// skip -> skipped (a different card instance).
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/cards/"+ci2.ID.String()+"/skip", nil)
	h.ServeHTTP(rr2, withCookie(req2, cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("skip: %d — %s", rr2.Code, rr2.Body.String())
	}
	var skipResp struct {
		CardStatus string `json:"card_status"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &skipResp); err != nil {
		t.Fatalf("decode skip resp: %v", err)
	}
	if skipResp.CardStatus != "skipped" {
		t.Fatalf("card_status = %q, want skipped", skipResp.CardStatus)
	}

	if got := count(`SELECT count(*) FROM graph_node`); got != nodesBefore {
		t.Fatalf("thin submit/skip must write no graph_node rows, before=%d after=%d", nodesBefore, got)
	}
	if got := count(`SELECT count(*) FROM graph_edge`); got != edgesBefore {
		t.Fatalf("thin submit/skip must write no graph_edge rows, before=%d after=%d", edgesBefore, got)
	}
	if got := count(`SELECT count(*) FROM intervention`); got != interventionsBefore {
		t.Fatalf("thin submit/skip must write no intervention rows, before=%d after=%d", interventionsBefore, got)
	}

	// ownership: another user's session's card 404s on submit/skip too.
	other := createStudent(t, pool, SeedSchoolID, "course-cardowner@demo.local")
	otherCookie := signInAs(t, pool, other)
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/cards/"+ci.ID.String()+"/skip", nil)
	h.ServeHTTP(rr3, withCookie(req3, otherCookie))
	if rr3.Code != http.StatusNotFound {
		t.Fatalf("cross-user card skip: want 404, got %d — %s", rr3.Code, rr3.Body.String())
	}
}

// TestCourseSession_ReloadSurfacesOpenCardOffer is the Critical-2 regression
// test: a card offer that exists in the DB (status proposed) but was never
// dispositioned must reappear in GET /session's openCards — this is exactly
// what a page reload during `guided` needs, since the offer otherwise lived
// only in React state and could never be re-offered, dead-ending the floor
// (card_dispositioned) forever. Mirrors mintPhaseCard's own invariant: the
// anchor material is created immediately before its card instance, in that
// order — the only creation path either row ever has for a course session,
// which is what lets OpenCardOffers pair them (card_instances has no
// material_id column).
func TestCourseSession_ReloadSurfacesOpenCardOffer(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	sessDTO := startSession(t, h, cookie, courseSeededCourseID)
	sessID := uuid.MustParse(sessDTO.ID)

	mat, err := q.CreateSessionMaterial(context.Background(), sqlc.CreateSessionMaterialParams{
		SessionID: pgUUID(sessID), Kind: "article", Source: "pasted", Title: "待核实的说法", Blocks: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	ci, err := q.CreateSessionCardInstance(context.Background(), sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(sessID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("get session: %d — %s", rr.Code, rr.Body.String())
	}
	var got CourseSessionDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.OpenCards) != 1 {
		t.Fatalf("openCards = %+v, want exactly 1 — the un-dispositioned offer must survive a reload", got.OpenCards)
	}
	oc := got.OpenCards[0]
	if oc.CardInstanceID != ci.ID.String() || oc.CardID != "craap" || oc.MaterialID != mat.ID.String() {
		t.Fatalf("openCards[0] = %+v, want cardInstanceId=%s cardId=craap materialId=%s", oc, ci.ID, mat.ID)
	}

	// Once dispositioned (skipped), it must not resurface — a resolved offer
	// stays resolved.
	if _, err := q.SetSessionCardInstanceStatus(context.Background(), sqlc.SetSessionCardInstanceStatusParams{
		ID: ci.ID, SessionID: pgUUID(sessID), Status: "skipped",
	}); err != nil {
		t.Fatalf("skip card: %v", err)
	}
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("get session after skip: %d — %s", rr2.Code, rr2.Body.String())
	}
	var got2 CourseSessionDTO
	if err := json.Unmarshal(rr2.Body.Bytes(), &got2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got2.OpenCards) != 0 {
		t.Fatalf("openCards after skip = %+v, want empty — a dispositioned card must not resurface", got2.OpenCards)
	}
}
