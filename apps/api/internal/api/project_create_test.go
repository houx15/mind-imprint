package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// composeJourneyStubProvider returns a canned model reply for
// agent.ComposeJourney's own gateway.Collect call — same scripted-provider
// shape as spotcheck_test.go's spotCheckStubProvider.
func composeJourneyStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 40, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// countJourneyComposedEvents counts the project's journey_composed events.
func countJourneyComposedEvents(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	rows, err := sqlc.New(pool).ListEventsByProject(context.Background(), pgtype.UUID{Bytes: mustUUID(projectID), Valid: true})
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	n := 0
	for _, r := range rows {
		if r.Type == "journey_composed" {
			n++
		}
	}
	return n
}

func TestCreateProject_SeedsOnboardingNodes(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论社交媒体对青少年注意力的影响"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	// The project exists, owned by the seed user, qualification 0457.
	var qual string
	if err := pool.QueryRow(context.Background(),
		`SELECT qualification FROM project WHERE id=$1`, pid).Scan(&qual); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if qual != "0457" {
		t.Errorf("qualification = %q, want 0457", qual)
	}
	// Exactly the three onboarding nodes, correct types + authors.
	rows, err := pool.Query(context.Background(),
		`SELECT type, author FROM graph_node WHERE project_id=$1 ORDER BY type`, pid)
	if err != nil {
		t.Fatalf("query nodes: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var typ, author string
		if err := rows.Scan(&typ, &author); err != nil {
			t.Fatal(err)
		}
		got[typ] = author
	}
	if got["assignment_brief"] != "imported" || got["rubric_translation"] != "ai" || got["milestone_plan"] != "ai" {
		t.Fatalf("onboarding nodes wrong: %+v", got)
	}
}

// The project title IS her research question — minted at creation so S1's
// read-only banner has a real node behind it and frame_question's
// node_present:research_question item has a producer at all.
func TestCreateProject_MintsResearchQuestion(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"社交媒体是否影响青少年注意力？","prompt":"讨论社交媒体对青少年注意力的影响"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	var author string
	var rqBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT author, body FROM graph_node WHERE project_id=$1 AND type='research_question'`, pid).
		Scan(&author, &rqBody); err != nil {
		t.Fatalf("research_question node missing: %v", err)
	}
	if author != "student" {
		t.Errorf("research_question author = %q, want student", author)
	}
	var got struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rqBody, &got); err != nil {
		t.Fatalf("unmarshal research_question body: %v; raw=%s", err, rqBody)
	}
	if got.Text != "社交媒体是否影响青少年注意力？" {
		t.Errorf("research_question text = %q, want the project title", got.Text)
	}
}

// #1 + #4: the creation selector's project type is stored as the display
// qualification (not the fixture code), and the chosen writing language is
// persisted as a writing_language graph node.
func TestCreateProject_StoresTypeAndWritingLanguage(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论语境与理解","projectType":"TOK 论文","writingLanguage":"en"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	var qual string
	if err := pool.QueryRow(context.Background(),
		`SELECT qualification FROM project WHERE id=$1`, pid).Scan(&qual); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if qual != "TOK 论文" {
		t.Errorf("qualification = %q, want the selected type 'TOK 论文'", qual)
	}

	var wlBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='writing_language'`, pid).Scan(&wlBody); err != nil {
		t.Fatalf("writing_language node missing: %v", err)
	}
	var wl struct {
		Lang string `json:"lang"`
	}
	if err := json.Unmarshal(wlBody, &wl); err != nil || wl.Lang != "en" {
		t.Fatalf("writing_language node lang = %q (err %v), want en", wl.Lang, err)
	}
}

// An absent projectType keeps the back-compat "0457" qualification, and an
// unknown writing language falls back to en.
func TestCreateProject_DefaultsWhenTypeAndLangAbsent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论语境与理解","writingLanguage":"martian"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	pid := uuid.MustParse(out.ID)

	var qual string
	if err := pool.QueryRow(context.Background(),
		`SELECT qualification FROM project WHERE id=$1`, pid).Scan(&qual); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if qual != "0457" {
		t.Errorf("qualification = %q, want 0457 fallback", qual)
	}
	var wlBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='writing_language'`, pid).Scan(&wlBody); err != nil {
		t.Fatalf("writing_language node missing: %v", err)
	}
	if !strings.Contains(string(wlBody), `"en"`) {
		t.Errorf("writing_language body = %s, want en fallback", wlBody)
	}
}

// project covers (Task 1): an explicit valid cover is stored verbatim.
func TestCreateProject_StoresExplicitCover(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论语境与理解","cover":"img:3"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	var cover string
	if err := pool.QueryRow(context.Background(),
		`SELECT cover FROM project WHERE id=$1`, pid).Scan(&cover); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if cover != "img:3" {
		t.Errorf("cover = %q, want img:3", cover)
	}
}

// project covers (Task 1): an absent/invalid cover falls back to a random
// valid photo cover — every project always has one.
func TestCreateProject_DefaultsRandomCoverWhenAbsent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论语境与理解"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	var cover string
	if err := pool.QueryRow(context.Background(),
		`SELECT cover FROM project WHERE id=$1`, pid).Scan(&cover); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if !strings.HasPrefix(cover, "img:") {
		t.Fatalf("cover = %q, want img:<N> default", cover)
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(cover, "img:"))
	if err != nil || idx < 1 || idx > 15 {
		t.Errorf("cover index = %q, want 1..15", cover)
	}
}

// GET /project-covers (Task 1): lists every pre-uploaded photo cover by key,
// regardless of whether OSS is wired (tests run with a.d.OSS == nil, so urls
// may legitimately be empty — assert the keys, which never depend on OSS).
func TestGetProjectCovers_ListsAllKeys(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/project-covers", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /project-covers = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Covers []struct {
			Key string `json:"key"`
			URL string `json:"url"`
		} `json:"covers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Covers) != 15 {
		t.Fatalf("covers count = %d, want 15", len(out.Covers))
	}
	for i, c := range out.Covers {
		want := "img:" + strconv.Itoa(i+1)
		if c.Key != want {
			t.Errorf("covers[%d].key = %q, want %q", i, c.Key, want)
		}
	}
}

func TestCreateProject_EmptyPromptRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{"prompt":""}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt = %d, want 400", rec.Code)
	}
}

// TestCreateProjectComposesJourney — N6-E Task 3: a canned decision list
// waiving decode_task/frame_question/evaluate_perspectives (keep=false) and
// keeping evaluate_sources/build_argument/draft_polish/reflect_archive
// (keep=true) — the "writing-project" skill's exact 7 contracts, as
// agent.ComposeJourney requires an EXACT cover — is persisted as the
// project's waived-set and recorded as one journey_composed event.
//
// NOTE (deviation from the brief): the projection does not yet render
// state:"waived" (that lands in Task 4), so this asserts the effect
// reachable today — a direct store.LoadWaived read plus the
// journey_composed event row — rather than GETting the projection.
func TestCreateProjectComposesJourney(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"id":"decode_task","keep":false,"reason":"她已经做过任务解码"},` +
		`{"id":"frame_question","keep":false,"reason":"已经有明确的研究问题"},` +
		`{"id":"evaluate_perspectives","keep":false,"reason":"已经列出了多个视角"},` +
		`{"id":"evaluate_sources","keep":true,"reason":"信源还没评估"},` +
		`{"id":"build_argument","keep":true,"reason":"论证还没搭"},` +
		`{"id":"draft_polish","keep":true,"reason":"成稿还没打磨"},` +
		`{"id":"reflect_archive","keep":true,"reason":"反思还没写"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     composeJourneyStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"我的论文","prompt":"我已经解码了任务、定好了研究问题、也列了三个视角，现在要开始评估信源了"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	waived, err := store.LoadWaived(context.Background(), uuid.MustParse(out.ID))
	if err != nil {
		t.Fatalf("LoadWaived: %v", err)
	}
	for _, id := range []string{"decode_task", "frame_question", "evaluate_perspectives"} {
		if !waived[id] {
			t.Errorf("waived[%q] = false, want true", id)
		}
	}
	for _, id := range []string{"evaluate_sources", "build_argument", "draft_polish", "reflect_archive"} {
		if waived[id] {
			t.Errorf("waived[%q] = true, want false (kept)", id)
		}
	}

	if n := countJourneyComposedEvents(t, pool, out.ID); n != 1 {
		t.Fatalf("journey_composed events = %d, want 1", n)
	}
	// The compose call is metered: creation must have recorded exactly one
	// "compose_journey" llm_call row for this project (not just a waived-set
	// side effect with no cost trail).
	if n := countLLMCallsByPurpose(t, pool, out.ID, "compose_journey"); n != 1 {
		t.Fatalf("compose_journey llm_call rows = %d, want 1", n)
	}
}

// TestCreateProjectFullJourneyWhenComposeFails — the composer's fail-safe:
// a malformed model reply must leave the full journey (no waived stations)
// and must NOT fail the create (still 201) — the project already exists by
// the time compose runs.
func TestCreateProjectFullJourneyWhenComposeFails(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     composeJourneyStubProvider("这不是 JSON，随便写点什么"),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"我的论文","prompt":"讨论社交媒体对青少年注意力的影响"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	waived, err := store.LoadWaived(context.Background(), uuid.MustParse(out.ID))
	if err != nil {
		t.Fatalf("LoadWaived: %v", err)
	}
	if len(waived) != 0 {
		t.Fatalf("waived set = %v, want empty (compose failed, full journey)", waived)
	}
	if n := countJourneyComposedEvents(t, pool, out.ID); n != 0 {
		t.Fatalf("journey_composed events = %d, want 0 (nothing to persist on fail-safe)", n)
	}
	// Metered even on reject: the malformed reply still cost a real model
	// call, so the compose_journey llm_call row must exist even though
	// nothing was persisted as a waived set (composeJourney's doc comment:
	// "every branch that made a real model call is metered").
	if n := countLLMCallsByPurpose(t, pool, out.ID, "compose_journey"); n != 1 {
		t.Fatalf("compose_journey llm_call rows = %d, want 1 (metered-on-reject)", n)
	}
}
