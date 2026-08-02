package api_test

// course_session_test.go — Task 7 (Slice 12): handler tests for the Course
// session surface (get-or-create, ownership, the SSE ask/advance turns, the
// phase floor's zero-LLM-call guarantee, thin card submit). Mirrors
// chat_test.go's testcontainers-backed pattern: real Postgres + a seeded
// student, a scripted JSON-emitting provider standing in for the model.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// TestCourseSession_Restart — restarting mints a NEW session id at the FIRST
// phase and wipes the prior run's session-scoped data (a card instance and a
// course_message row created against the old session id are gone after
// restart — the ON DELETE CASCADE, not a soft reset). It also proves the
// user+course-scoped course_progress row (NOT session-scoped, so the cascade
// never touches it — Task 5's fix) is reset by the handler itself: without
// the explicit DeleteCourseProgressByUserCourse call, CoursePlayer would
// resume the content pane at the last-viewed ordinal after a restart.
func TestCourseSession_Restart(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     courseProvider(`{"type":"advance","to":"guided"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	sess := startSession(t, h, cookie, courseSeededCourseID)
	sessID := uuid.MustParse(sess.ID)

	// Create session-scoped state to prove restart wipes it: a card instance
	// and a course_message row against the pre-restart session id.
	ci, err := q.CreateSessionCardInstance(context.Background(), sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(sessID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}
	if _, err := q.CreateCourseMessage(context.Background(), sqlc.CreateCourseMessageParams{
		SessionID: sessID, Phase: "demonstrate", Role: "student", Content: "在重启前留下的一句话",
	}); err != nil {
		t.Fatalf("create course message: %v", err)
	}

	// Advance the session's phase away from the first phase (demonstrate is
	// unreachable to satisfy here without rendering steps, so instead just
	// bump the DB row directly to prove restart resets phase, not merely
	// preserves whatever it already was).
	if _, err := q.SetCourseSessionPhase(context.Background(), sqlc.SetCourseSessionPhaseParams{
		ID: sessID, Phase: "guided",
	}); err != nil {
		t.Fatalf("bump phase: %v", err)
	}

	// Write a non-zero course_progress row (page position) to prove restart
	// resets it too, not just the session's phase.
	courseUUID := uuid.MustParse(courseSeededCourseID)
	if _, err := q.UpsertCourseProgress(context.Background(), sqlc.UpsertCourseProgressParams{
		UserID: SeedUserID, CourseID: courseUUID, CurrentOrdinal: 3, CompletedOrdinals: []int32{0, 1, 2},
	}); err != nil {
		t.Fatalf("seed course progress: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/restart", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("restart = %d; %s", rec.Code, rec.Body)
	}
	var fresh CourseSessionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &fresh); err != nil {
		t.Fatalf("decode restart response: %v", err)
	}
	if fresh.ID == sess.ID {
		t.Fatalf("restart must mint a NEW session id, got the same %s", fresh.ID)
	}
	if fresh.Phase != "demonstrate" {
		t.Fatalf("restart phase = %q, want the first phase demonstrate", fresh.Phase)
	}
	if len(fresh.CollectedCards) != 0 {
		t.Fatalf("fresh session must carry no collected cards, got %+v", fresh.CollectedCards)
	}

	// The old session row itself is gone (cascade deletes it, not just its
	// children) — the get-or-create resolves to fresh.ID from here on.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM course_session WHERE id = $1`, sessID,
	).Scan(&count); err != nil {
		t.Fatalf("count old session row: %v", err)
	}
	if count != 0 {
		t.Fatalf("old session row must be deleted by restart, found %d", count)
	}

	// The old session's card instance and course_message rows are wiped by
	// the cascade — session-scoped state does not survive a restart.
	var ciCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM card_instances WHERE id = $1`, ci.ID,
	).Scan(&ciCount); err != nil {
		t.Fatalf("count old card instance: %v", err)
	}
	if ciCount != 0 {
		t.Fatalf("old card instance must be wiped by the cascade, found %d", ciCount)
	}
	var msgCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM course_message WHERE session_id = $1`, sessID,
	).Scan(&msgCount); err != nil {
		t.Fatalf("count old course_message rows: %v", err)
	}
	if msgCount != 0 {
		t.Fatalf("old course_message rows must be wiped by the cascade, found %d", msgCount)
	}

	// course_progress is NOT session-scoped (migration 0023 leaves it
	// untouched), so it does NOT ride the cascade above — the handler must
	// reset it explicitly. Either no row remains, or it's back at the
	// table defaults (current_ordinal 0, empty completed_ordinals).
	prog, err := q.GetCourseProgress(context.Background(), sqlc.GetCourseProgressParams{
		UserID: SeedUserID, CourseID: courseUUID,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("get course progress after restart: %v", err)
		}
	} else if prog.CurrentOrdinal != 0 || len(prog.CompletedOrdinals) != 0 {
		t.Fatalf("restart must reset course_progress (page position), got current_ordinal=%d completed_ordinals=%v",
			prog.CurrentOrdinal, prog.CompletedOrdinals)
	}

	// GET session now resolves to the fresh session, not the old one.
	got := startSession(t, h, cookie, courseSeededCourseID)
	if got.ID != fresh.ID {
		t.Fatalf("subsequent get-or-create = %s, want the restarted session %s", got.ID, fresh.ID)
	}
}

// TestCourseSession_Restart_OwnershipDeletesNothing mirrors
// TestCourseSession_OwnershipIs404: restarting deletes ONLY the caller's own
// session (keyed by user_id), never another student's — a caller with no
// session of their own for this course still gets 200 with a freshly minted
// session, and the other student's session/card survive untouched.
func TestCourseSession_Restart_OwnershipDeletesNothing(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, SpecByID: cards.ByID}).Handler()
	courseID := uuid.MustParse(courseSeededCourseID)

	other := createStudent(t, pool, SeedSchoolID, "course-restart-other@demo.local")
	otherSess, err := q.CreateCourseSession(context.Background(), sqlc.CreateCourseSessionParams{
		UserID: other, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create other student's session: %v", err)
	}

	cookie := signInSeed(t, pool) // Phoebe — has no session of her own yet.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/restart", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("restart with no prior session = %d; %s", rec.Code, rec.Body)
	}
	var fresh CourseSessionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &fresh); err != nil {
		t.Fatalf("decode restart response: %v", err)
	}
	if fresh.ID == otherSess.ID.String() {
		t.Fatal("the caller's fresh session must not be the other student's session")
	}

	// The other student's session must be untouched.
	var otherStillThere int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM course_session WHERE id = $1`, otherSess.ID,
	).Scan(&otherStillThere); err != nil {
		t.Fatalf("count other student's session: %v", err)
	}
	if otherStillThere != 1 {
		t.Fatal("another student's session must survive the caller's restart untouched")
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

// TestGetCourseSession_CarriesCollectedCards — A1: the report's 收集到的工具
// block needs the session's COMPLETED cards. openCards deliberately carries
// only undispositioned offers, so a completed card would otherwise never
// reach the client. A skipped card is NOT collected — the student declined
// it, and saying otherwise would be a fabrication.
func TestGetCourseSession_CarriesCollectedCards(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	q := sqlc.New(pool)
	ctx := context.Background()
	sess, err := q.GetCourseSessionByUserCourse(ctx, sqlc.GetCourseSessionByUserCourseParams{
		UserID: SeedUserID, CourseID: uuid.MustParse(courseSeededCourseID),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	sid := pgUUID(sess.ID)
	for _, c := range []struct{ cardID, status string }{
		{"craap", "completed"},
		{"concession", "skipped"},
		{"toulmin", "proposed"},
	} {
		if _, err := q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
			SessionID: sid, CardID: c.cardID, Status: c.status,
		}); err != nil {
			t.Fatalf("create %s: %v", c.cardID, err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET session = %d; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		CollectedCards []struct {
			CardID string `json:"cardId"`
		} `json:"collectedCards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(dto.CollectedCards) != 1 || dto.CollectedCards[0].CardID != "craap" {
		t.Fatalf("collectedCards = %+v, want only the completed craap — a skipped or still-proposed card is not collected", dto.CollectedCards)
	}
}

// TestGetCourseSession_DedupsRepeatedCompletedCard — the SAME cardId
// completed twice (e.g. the student re-summoned and re-finished the same
// tool card within one session) must collapse to exactly one collected
// entry: "the same tool completed twice is one collected tool," not two.
func TestGetCourseSession_DedupsRepeatedCompletedCard(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, courseSeededCourseID)

	q := sqlc.New(pool)
	ctx := context.Background()
	sess, err := q.GetCourseSessionByUserCourse(ctx, sqlc.GetCourseSessionByUserCourseParams{
		UserID: SeedUserID, CourseID: uuid.MustParse(courseSeededCourseID),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	sid := pgUUID(sess.ID)
	for i := 0; i < 2; i++ {
		if _, err := q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
			SessionID: sid, CardID: "craap", Status: "completed",
		}); err != nil {
			t.Fatalf("create craap completion %d: %v", i, err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET session = %d; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		CollectedCards []struct {
			CardID string `json:"cardId"`
		} `json:"collectedCards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(dto.CollectedCards) != 1 || dto.CollectedCards[0].CardID != "craap" {
		t.Fatalf("collectedCards = %+v, want exactly one deduped craap entry — the same card_id completed twice must collapse to one collected tool", dto.CollectedCards)
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

// TestCourseSession_WalkFinishesAtChallenge is the N5c Task-2 course-walk
// regression: drives the REAL seeded skill (info-literacy-course) through
// every phase — demonstrate → guided → independent → reflect → challenge —
// satisfying each phase's floor along the way, exactly as
// TestCourseSession_Ask / TestCourseSession_AdvanceMovesThePhase do per-phase.
// Task 1 appended challenge (练一手, floor: []) after reflect; this proves the
// terminal moved there for real (not just in NextPhase's unit test): the FINAL
// advance — from challenge, whose empty floor is always met — must set
// status=finished and emit course_finished, with zero regressions to the
// earlier phases' floors along the way.
func TestCourseSession_WalkFinishesAtChallenge(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	cookie := signInSeed(t, pool)

	newHandler := func(p gateway.Provider) http.Handler {
		return New(Deps{
			Queries: q, Pool: pool, Provider: p, ChatResolver: fakeResolver(), EvalResolver: fakeResolver(), SpecByID: cards.ByID,
		}).Handler()
	}
	advanceTo := func(h http.Handler, wantPhase string) {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("advance to %s: %d — %s", wantPhase, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if !strings.Contains(body, "event: phase") || !strings.Contains(body, `"to":"`+wantPhase+`"`) {
			t.Fatalf("advance to %s: expected a phase frame:\n%s", wantPhase, body)
		}
		var phase string
		if err := pool.QueryRow(context.Background(),
			`SELECT phase FROM course_session WHERE user_id = $1 AND course_id = $2`, SeedUserID, courseSeededCourseID,
		).Scan(&phase); err != nil {
			t.Fatalf("query session phase: %v", err)
		}
		if phase != wantPhase {
			t.Fatalf("phase = %q, want %q", phase, wantPhase)
		}
	}
	ask := func(h http.Handler, msg string) {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/ask",
			strings.NewReader(`{"user_input":"`+msg+`"}`)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("ask: %d — %s", rr.Code, rr.Body.String())
		}
	}

	startSession(t, newHandler(nil), cookie, courseSeededCourseID)

	// demonstrate: steps_viewed [0,1] — render both ordinals, then advance.
	// RenderCourseStep needs a non-nil Provider even though its output is
	// discarded here (a plain "reply" JSON fails renderTeaching's stricter
	// title/subtitle/body parse and falls back to the authored content, which
	// is all this walk needs — it only cares that the render endpoint marks
	// the ordinal viewed).
	hRender := newHandler(courseProvider(`{"type":"reply","body":"ok"}`))
	for _, ord := range []int{0, 1} {
		rr := httptest.NewRecorder()
		hRender.ServeHTTP(rr, withCookie(httptest.NewRequest("POST",
			"/api/v1/courses/"+courseSeededCourseID+"/steps/"+strconv.Itoa(ord)+"/render", nil), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("render step %d: %d — %s", ord, rr.Code, rr.Body.String())
		}
	}
	advanceTo(newHandler(courseProvider(`{"type":"advance","to":"guided"}`)), "guided")

	// guided: card_dispositioned craap — first advance is refused (no card
	// dispositioned yet) but mints the phase's card offer via mintPhaseCard's
	// refusal path; fetch it off GET session, skip it, then advance clears.
	hGuided := newHandler(courseProvider(`{"type":"advance","to":"independent"}`))
	rrRefuse := httptest.NewRecorder()
	hGuided.ServeHTTP(rrRefuse, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
	if rrRefuse.Code != http.StatusOK {
		t.Fatalf("guided refusal advance: %d — %s", rrRefuse.Code, rrRefuse.Body.String())
	}
	if strings.Contains(rrRefuse.Body.String(), "event: phase") {
		t.Fatalf("guided's card_dispositioned floor is unmet — must not advance yet:\n%s", rrRefuse.Body.String())
	}
	rrGet := httptest.NewRecorder()
	hGuided.ServeHTTP(rrGet, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+courseSeededCourseID+"/session", nil), cookie))
	if rrGet.Code != http.StatusOK {
		t.Fatalf("get session: %d — %s", rrGet.Code, rrGet.Body.String())
	}
	var sessDTO CourseSessionDTO
	if err := json.Unmarshal(rrGet.Body.Bytes(), &sessDTO); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if len(sessDTO.OpenCards) != 1 {
		t.Fatalf("openCards = %+v, want exactly 1 (minted by the refusal path)", sessDTO.OpenCards)
	}
	skipRR := httptest.NewRecorder()
	hGuided.ServeHTTP(skipRR, withCookie(httptest.NewRequest("POST",
		"/api/v1/courses/"+courseSeededCourseID+"/session/cards/"+sessDTO.OpenCards[0].CardInstanceID+"/skip", nil), cookie))
	if skipRR.Code != http.StatusOK {
		t.Fatalf("skip card: %d — %s", skipRR.Code, skipRR.Body.String())
	}
	advanceTo(hGuided, "independent")

	// independent: student_turns_at_least 1 — one ask, then advance.
	hIndependent := newHandler(courseProvider(`{"type":"reply","body":"你的理由是什么？"}`))
	ask(hIndependent, "我觉得不确定，因为证据不够")
	advanceTo(newHandler(courseProvider(`{"type":"advance","to":"reflect"}`)), "reflect")

	// reflect: student_turns_at_least 1 — one ask, then advance to challenge —
	// the phase Task 1 appended, now the terminal.
	hReflect := newHandler(courseProvider(`{"type":"reply","body":"说说你学到的方法。"}`))
	ask(hReflect, "这次我学会了先横向溯源再下判断")
	advanceTo(newHandler(courseProvider(`{"type":"advance","to":"challenge"}`)), "challenge")

	// challenge: floor: [] — an empty floor must always be met, with no
	// dispositioned card and no student turn in this phase at all. The final
	// advance is the terminal: NextPhase(challenge) has no successor, so
	// runCourseAdvance's terminal branch fires — a pure store operation, zero
	// model calls — setting status=finished and emitting course_finished.
	hFinal := newHandler(courseProvider(`{"type":"advance","to":"nowhere"}`)) // unreachable: the terminal returns before any model call
	rrFinal := httptest.NewRecorder()
	hFinal.ServeHTTP(rrFinal, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseSeededCourseID+"/session/advance", nil), cookie))
	if rrFinal.Code != http.StatusOK {
		t.Fatalf("terminal advance: %d — %s", rrFinal.Code, rrFinal.Body.String())
	}
	finalBody := rrFinal.Body.String()
	if !strings.Contains(finalBody, "event: text") {
		t.Fatalf("terminal advance: expected a text frame:\n%s", finalBody)
	}
	if strings.Contains(finalBody, "event: phase") {
		t.Fatalf("challenge has no successor — must NOT emit a phase frame:\n%s", finalBody)
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM course_session WHERE user_id = $1 AND course_id = $2`, SeedUserID, courseSeededCourseID,
	).Scan(&status); err != nil {
		t.Fatalf("query session status: %v", err)
	}
	if status != "finished" {
		t.Fatalf("status = %q, want finished", status)
	}

	var finishedEvents int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE surface = 'course' AND type = 'course_finished' AND session_id = (
			SELECT id FROM course_session WHERE user_id = $1 AND course_id = $2
		)`, SeedUserID, courseSeededCourseID,
	).Scan(&finishedEvents); err != nil {
		t.Fatalf("count course_finished events: %v", err)
	}
	if finishedEvents != 1 {
		t.Fatalf("course_finished events = %d, want exactly 1", finishedEvents)
	}
}
