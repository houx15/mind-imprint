package api_test

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

// One transaction replaces the whole S1 set, so editing a definition never
// leaves a duplicate node behind.
func TestSubmitFraming_ReplacesNodes(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	body := `{
		"terms":[{"term":"可持续发展","definition":"资源使用不损害后代人的需求满足能力"}],
		"answers":["中国的可再生能源投入让全球减排更快"],
		"searchPlan":["官方一手数据来源"]
	}`
	submit := func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
			strings.NewReader(body)), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
		}
	}
	submit()
	submit() // re-submit must not inflate counts

	for _, typ := range []string{"term_definition", "provisional_answer", "preregistration"} {
		var count int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type=$2`, pid, typ).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", typ, err)
		}
		if count != 1 {
			t.Errorf("%s nodes after two submits = %d, want 1", typ, count)
		}
	}

	var termBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='term_definition'`, pid).Scan(&termBody); err != nil {
		t.Fatalf("select term_definition body: %v", err)
	}
	var gotTerm struct {
		Term       string `json:"term"`
		Definition string `json:"definition"`
		Origin     string `json:"origin"`
	}
	if err := json.Unmarshal(termBody, &gotTerm); err != nil {
		t.Fatalf("unmarshal term_definition body: %v; raw=%s", err, termBody)
	}
	if gotTerm.Term != "可持续发展" || gotTerm.Origin != "station_view" {
		t.Errorf("term_definition body = %+v, want term=可持续发展 origin=station_view", gotTerm)
	}

	var preregBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='preregistration'`, pid).Scan(&preregBody); err != nil {
		t.Fatalf("select preregistration body: %v", err)
	}
	var gotPrereg struct {
		Directions []string `json:"directions"`
		Origin     string   `json:"origin"`
	}
	if err := json.Unmarshal(preregBody, &gotPrereg); err != nil {
		t.Fatalf("unmarshal preregistration body: %v; raw=%s", err, preregBody)
	}
	if len(gotPrereg.Directions) != 1 || gotPrereg.Directions[0] != "官方一手数据来源" || gotPrereg.Origin != "station_view" {
		t.Errorf("preregistration body = %+v, want directions=[官方一手数据来源] origin=station_view", gotPrereg)
	}
}

// terms_defined is recorded once three definitions clear 15 runes — and is
// CLEARED again when one is emptied, so the gate never reports work that is
// no longer there.
func TestSubmitFraming_AttestsAndUnattestsTermsDefined(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	// Three CJK definitions, each clearing 15 runes (byte length would be ~3x
	// this and could pass a broken byte-count check on much shorter text —
	// keep these comfortably over 15 runes either way).
	full := `{
		"terms":[
			{"term":"可持续发展","definition":"资源使用不损害后代人的需求满足能力这是环境层面的定义"},
			{"term":"中国的角色","definition":"中国政策与产出对全球环境指标的净影响这是国家层面的定义"},
			{"term":"世界","definition":"全球尺度而非仅中国境内的地理范围这是空间层面的定义"}
		],
		"answers":["中国的可再生能源投入让全球减排更快"],
		"searchPlan":["官方一手数据来源"]
	}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(full)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	gateItems := func() map[string]string {
		var gateBody []byte
		if err := pool.QueryRow(context.Background(),
			`SELECT body FROM graph_node WHERE project_id=$1 AND type='gate_state' AND body->>'contract'='frame_question'`, pid).
			Scan(&gateBody); err != nil {
			t.Fatalf("select frame_question gate_state: %v", err)
		}
		var got struct {
			Items map[string]string `json:"items"`
		}
		if err := json.Unmarshal(gateBody, &got); err != nil {
			t.Fatalf("unmarshal gate_state body: %v; raw=%s", err, gateBody)
		}
		return got.Items
	}

	if items := gateItems(); items["terms_defined"] != "solid" {
		t.Fatalf("frame_question gate items[terms_defined] = %q, want solid", items["terms_defined"])
	}

	// Re-submit with one definition emptied: terms_defined must un-attest.
	partial := `{
		"terms":[
			{"term":"可持续发展","definition":""},
			{"term":"中国的角色","definition":"中国政策与产出对全球环境指标的净影响这是国家层面的定义"},
			{"term":"世界","definition":"全球尺度而非仅中国境内的地理范围这是空间层面的定义"}
		],
		"answers":["中国的可再生能源投入让全球减排更快"],
		"searchPlan":["官方一手数据来源"]
	}`
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(partial)), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("re-submit = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}

	if items := gateItems(); items["terms_defined"] != "" {
		t.Errorf("frame_question gate items[terms_defined] = %q after emptying a definition, want absent", items["terms_defined"])
	}
}

// A half-finished S1 saves fine (an offer is never a wall) — blank rows are
// dropped, not rejected.
func TestSubmitFraming_SavesPartialWork(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	body := `{
		"terms":[{"term":"可持续发展","definition":""},{"term":"","definition":""}],
		"answers":["", "  "],
		"searchPlan":["", "  "]
	}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("partial submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var termCount, answerCount, preregCount int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='term_definition'`, pid).Scan(&termCount)
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='provisional_answer'`, pid).Scan(&answerCount)
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='preregistration'`, pid).Scan(&preregCount)

	if termCount != 1 {
		t.Errorf("term_definition nodes = %d, want 1 (blank term row dropped)", termCount)
	}
	if answerCount != 0 {
		t.Errorf("provisional_answer nodes = %d, want 0 (all blank)", answerCount)
	}
	if preregCount != 0 {
		t.Errorf("preregistration nodes = %d, want 0 (all blank directions)", preregCount)
	}

	// Empty request entirely: must also succeed and simply delete everything.
	empty := `{"terms":[],"answers":[],"searchPlan":[]}`
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(empty)), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("empty submit = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	var total int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type=ANY(ARRAY['term_definition','provisional_answer','preregistration'])`, pid).
		Scan(&total)
	if total != 0 {
		t.Errorf("nodes after empty submit = %d, want 0", total)
	}
}
