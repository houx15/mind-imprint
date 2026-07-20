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

func TestSubmitSelfScore_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/self-score",
		strings.NewReader(`{"scores":[{"code":"表D","band":2},{"code":"表E","band":0}]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("self-score = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='self_score' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("self_score nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='self_scored'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("self_scored events = %d, want 1", events)
	}
	// body content — catches a code/band field drift. Unmarshal rather than
	// substring-match the raw jsonb text: Postgres's jsonb canonical output
	// reformats whitespace (e.g. `"band": 2`, not `"band":2`), so a literal
	// substring check on stored jsonb is brittle.
	var body []byte
	_ = pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='self_score'`, pid).Scan(&body)
	var got struct {
		Scores []struct {
			Code string `json:"code"`
			Band int    `json:"band"`
		} `json:"scores"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal self_score body: %v; raw=%s", err, body)
	}
	want := []struct {
		Code string `json:"code"`
		Band int    `json:"band"`
	}{{Code: "表D", Band: 2}, {Code: "表E", Band: 0}}
	if !reflect.DeepEqual(got.Scores, want) {
		t.Errorf("self_score body scores = %+v, want %+v", got.Scores, want)
	}
}

func TestSubmitSelfScore_RejectsBadBandOrCode(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	for _, bad := range []string{`{"scores":[{"code":"表D","band":9}]}`, `{"scores":[{"code":"表Z","band":1}]}`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/self-score", strings.NewReader(bad)), cookie))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s = %d, want 400", bad, rec.Code)
		}
	}
}

func TestSubmitSelfScore_OtherUsersProject404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	other := createStudent(t, pool, SeedSchoolID, "selfscore-other@demo.local")
	otherCookie := signInAs(t, pool, other)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/self-score",
		strings.NewReader(`{"scores":[{"code":"表D","band":1}]}`)), otherCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other user's self-score submit = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
