package api_test

// walkable_stations_test.go — Task 13 (N3d): the acceptance test that could
// not have passed before this slice at all.
//
// Before N3d, walking S0->S3 was impossible twice over:
//  1. agent.Advance — the only function that can set a gate's Confirmed flag,
//     which studio.projectStations reads as GateReport.Solid — had ZERO
//     production call sites. It was called from tests only, so no station
//     could ever become "done" for a project a real student created.
//  2. S0/S1/S2 between them had six gate items with no producer at all
//     (weakness_prediction, milestone_plan, research_question,
//     provisional_answer, preregistration, terms_defined).
//
// Nobody noticed because internal/store/migrations/0018_seed_demo_project.sql
// hand-writes confirmed_solid:true gate_state rows for the seeded demo
// project's first four contracts — it LOOKS walkable because a migration
// says so, not because anything computed it. This test therefore creates its
// own project through the real funnel and never touches the seeded one: if it
// touched the seed, it would prove nothing.
//
// It asserts on the PROJECTION's station states at each step (GET
// /api/v1/projects/{id}, the `stations[].state` field) — that is what the
// student actually sees in the rail — never on gate internals directly.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// stationsSnapshot is the slice of the projection this test cares about.
type stationsSnapshot struct {
	Stations []struct {
		Code  string `json:"code"`
		State string `json:"state"`
	} `json:"stations"`
}

func fetchStations(t *testing.T, h http.Handler, pid string, cookie *http.Cookie) stationsSnapshot {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET project = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out stationsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal projection: %v; raw=%s", err, rec.Body.Bytes())
	}
	return out
}

func stationState(t *testing.T, snap stationsSnapshot, code string) string {
	t.Helper()
	for _, st := range snap.Stations {
		if st.Code == code {
			return st.State
		}
	}
	t.Fatalf("no station with code %q in projection; stations=%+v", code, snap.Stations)
	return ""
}

// TestWalkableStations_S0ToS3 drives a freshly created project all the way
// from S0 to S3 using nothing but the student-facing HTTP endpoints, and
// checks the station rail turns "done"/"current" honestly at each step.
func TestWalkableStations_S0ToS3(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	// --- POST /projects: research_question exists, S0 is current -------
	pid := createProjectForTest(t, h, cookie)

	var rqCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='research_question'`, pid).
		Scan(&rqCount); err != nil {
		t.Fatalf("count research_question: %v", err)
	}
	if rqCount != 1 {
		t.Fatalf("research_question nodes after create = %d, want 1", rqCount)
	}

	snap := fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S0"); got != "current" {
		t.Fatalf("S0 state after create = %q, want current", got)
	}

	// --- POST /onboarding (2 weak picks): decode_task solid, S1 current -
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问中国是否让地球变得更可持续","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit onboarding = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S0"); got != "done" {
		t.Fatalf("S0 state after onboarding submit = %q, want done", got)
	}
	if got := stationState(t, snap, "S1"); got != "current" {
		t.Fatalf("S1 state after onboarding submit = %q, want current", got)
	}

	// --- POST /framing (3 terms >=15 runes, 1 answer, 1 search direction) -
	// frame_question solid, S2 current.
	framingBody := `{
		"terms":[
			{"term":"可持续发展","definition":"资源使用不损害后代人满足自身需求的能力这是环境层面的定义"},
			{"term":"中国的角色","definition":"中国的政策与产出对全球环境指标造成的净影响这是国家层面的定义"},
			{"term":"世界","definition":"全球尺度而非仅中国境内的地理与生态范围这是空间层面的定义"}
		],
		"answers":["中国的可再生能源投入使全球减排速度整体加快"],
		"searchPlan":["官方一手排放与能源数据来源"]
	}`
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(framingBody)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit framing = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S1"); got != "done" {
		t.Fatalf("S1 state after framing submit = %q, want done", got)
	}
	if got := stationState(t, snap, "S2"); got != "current" {
		t.Fatalf("S2 state after framing submit = %q, want current", got)
	}

	// --- POST /materials + /materials/{mid}/open: records recon_logged ---
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/materials",
		strings.NewReader(`{"title":"NASA 绿化研究","text":"叶面积指数在2000到2017年间上升了约5%，中国和印度贡献了净增量的三分之一。","takeaway":"变绿是真的，但不等于可持续。","tier":"一手数据"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingest material = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var mat struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mat); err != nil {
		t.Fatalf("unmarshal material response: %v; raw=%s", err, rec.Body.Bytes())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/materials/"+mat.ID+"/open",
		strings.NewReader(`{"time_spent_s":30}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("log source open = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	var reconLoggedItem string
	if err := pool.QueryRow(context.Background(),
		`SELECT body->'items'->>'recon_logged' FROM graph_node WHERE project_id=$1 AND type='gate_state' AND body->>'contract'='evaluate_perspectives'`, pid).
		Scan(&reconLoggedItem); err != nil {
		t.Fatalf("select evaluate_perspectives gate_state after source open: %v", err)
	}
	if reconLoggedItem != "solid" {
		t.Fatalf("recon_logged item after logging a source open = %q, want solid", reconLoggedItem)
	}

	// --- POST /perspectives (2 rows, both layers) ------------------------
	perspectivesBody := `{"perspectives":[
		{"text":"中国国内视角：可再生能源产业规模全球第一","level":"national"},
		{"text":"全球视角：中国碳排放总量仍是世界第一，抵消了绿化收益","level":"global_against"}
	]}`
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
		strings.NewReader(perspectivesBody)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit perspectives = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// S2 should still be current: 2 perspectives + recon_logged are in, but
	// sources_per_perspective has not been explicitly attested yet.
	snap = fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S2"); got != "current" {
		t.Fatalf("S2 state after perspectives submit (before attest) = %q, want current", got)
	}

	// --- POST /gate/evaluate_perspectives/attest {sources_per_perspective} -
	// evaluate_perspectives solid, S3 current.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/gate/evaluate_perspectives/attest",
		strings.NewReader(`{"item":"sources_per_perspective","confirmed":true}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attest sources_per_perspective = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S2"); got != "done" {
		t.Fatalf("S2 state after sources_per_perspective attest = %q, want done", got)
	}
	if got := stationState(t, snap, "S3"); got != "current" {
		t.Fatalf("S3 state after sources_per_perspective attest = %q, want current", got)
	}
}
