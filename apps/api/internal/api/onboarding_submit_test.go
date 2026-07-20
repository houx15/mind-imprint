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

func createProjectForTest(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"T","prompt":"某个任务要求"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct{ ID string `json:"id"` }
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
