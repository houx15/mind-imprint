package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func createProjectForTest(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"T","prompt":"某个任务要求"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func TestSubmitOnboarding_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='task_restatement' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("task_restatement nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='onboarding_restated'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("onboarding_restated events = %d, want 1", events)
	}

	// Assert body content, not just row counts — catches field-name drift
	// (e.g. weakPicks vs weak_picks) that a count-only assertion can't see.
	var nodeBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='task_restatement' AND author='student'`, pid).
		Scan(&nodeBody); err != nil {
		t.Fatalf("select node body: %v", err)
	}
	var gotNode struct {
		Restate   string `json:"restate"`
		WeakPicks []int  `json:"weak_picks"`
	}
	if err := json.Unmarshal(nodeBody, &gotNode); err != nil {
		t.Fatalf("unmarshal node body: %v; raw=%s", err, nodeBody)
	}
	if gotNode.Restate != "这道题在问社交媒体是否影响注意力" || !reflect.DeepEqual(gotNode.WeakPicks, []int{0, 2}) {
		t.Errorf("node body = %+v, want restate=%q weak_picks=[0 2]", gotNode, "这道题在问社交媒体是否影响注意力")
	}

	var eventPayload []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT payload FROM event WHERE project_id=$1 AND type='onboarding_restated'`, pid).
		Scan(&eventPayload); err != nil {
		t.Fatalf("select event payload: %v", err)
	}
	var gotEvent struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(eventPayload, &gotEvent); err != nil {
		t.Fatalf("unmarshal event payload: %v; raw=%s", err, eventPayload)
	}
	if gotEvent.Text != "这道题在问社交媒体是否影响注意力" {
		t.Errorf("event payload text = %q, want %q", gotEvent.Text, "这道题在问社交媒体是否影响注意力")
	}
}

// Each weak pick becomes a weakness_prediction node so decode_task's
// node_count_at_least{weakness_prediction,2} has a producer.
func TestSubmitOnboarding_MintsWeaknessPredictions(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rows, err := pool.Query(context.Background(),
		`SELECT body, author FROM graph_node WHERE project_id=$1 AND type='weakness_prediction' ORDER BY body->>'index'`, pid)
	if err != nil {
		t.Fatalf("query weakness_prediction nodes: %v", err)
	}
	defer rows.Close()
	var got []struct {
		Index  int    `json:"index"`
		Plain  string `json:"plain"`
		Origin string `json:"origin"`
	}
	var author string
	for rows.Next() {
		var body []byte
		var g struct {
			Index  int    `json:"index"`
			Plain  string `json:"plain"`
			Origin string `json:"origin"`
		}
		if err := rows.Scan(&body, &author); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &g); err != nil {
			t.Fatalf("unmarshal weakness_prediction body: %v; raw=%s", err, body)
		}
		got = append(got, g)
	}
	if len(got) != 2 {
		t.Fatalf("weakness_prediction nodes = %d, want 2 (got %+v)", len(got), got)
	}
	if author != "student" {
		t.Errorf("weakness_prediction author = %q, want student", author)
	}
	for _, g := range got {
		if g.Origin != "station_view" {
			t.Errorf("weakness_prediction origin = %q, want station_view", g.Origin)
		}
		if g.Plain == "" {
			t.Errorf("weakness_prediction index %d has empty plain label", g.Index)
		}
	}
}

// Re-submitting must not inflate the count past the n>=2 gate: two submits of
// ONE pick leave exactly one node, not two. (Without the origin-scoped delete
// a student could clear the gate by pressing save twice.)
func TestSubmitOnboarding_ReplacesWeaknessPredictions(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	submit := func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
			strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[1]}`)), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
		}
	}
	submit()
	submit()

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='weakness_prediction'`, pid).Scan(&count); err != nil {
		t.Fatalf("count weakness_prediction nodes: %v", err)
	}
	if count != 1 {
		t.Fatalf("weakness_prediction nodes after two submits of one pick = %d, want 1", count)
	}
}

// milestone_plan is recorded on her submit, like reflection.go records its own.
func TestSubmitOnboarding_AttestsMilestonePlan(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var gateBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='gate_state' AND body->>'contract'='decode_task'`, pid).
		Scan(&gateBody); err != nil {
		t.Fatalf("select decode_task gate_state: %v", err)
	}
	var gotGate struct {
		Items map[string]string `json:"items"`
	}
	if err := json.Unmarshal(gateBody, &gotGate); err != nil {
		t.Fatalf("unmarshal gate_state body: %v; raw=%s", err, gateBody)
	}
	if gotGate.Items["milestone_plan"] != "solid" {
		t.Errorf("decode_task gate items[milestone_plan] = %q, want solid", gotGate.Items["milestone_plan"])
	}

	// S0's gate should now be fully closed: rubric_translation node present
	// (from creation), 2 weakness_prediction nodes, milestone_plan attested —
	// advanceGates (already wired at the end of submitOnboarding) should have
	// flipped decode_task's recorded state to confirmed_solid.
	var confirmed bool
	if err := pool.QueryRow(context.Background(),
		`SELECT (body->>'confirmed_solid')::bool FROM graph_node WHERE project_id=$1 AND type='gate_state' AND body->>'contract'='decode_task'`, pid).
		Scan(&confirmed); err != nil {
		t.Fatalf("select decode_task gate_state confirmed_solid: %v", err)
	}
	if !confirmed {
		t.Errorf("decode_task confirmed_solid = false, want true (gate should be closed)")
	}
}

func TestSubmitOnboarding_OtherUsersProject404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	other := createStudent(t, pool, SeedSchoolID, "onboard-other@demo.local")
	otherCookie := signInAs(t, pool, other)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[0,2]}`)), otherCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other user's onboarding submit = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

func TestSubmitOnboarding_ShortRestateRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"太短","weakPicks":[]}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short restate = %d, want 400", rec.Code)
	}
}
