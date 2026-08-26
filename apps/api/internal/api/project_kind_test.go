package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// insertContainerProject inserts a raw container-kind project row for the seed
// user, bypassing the API (nothing creates containers yet — that lands in
// Task 4). Returns its id.
func insertContainerProject(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO project (user_id, qualification, title, board_cfg_ver, kind)
		 VALUES ($1, '0457', '容器', 1, 'container') RETURNING id::text`,
		SeedUserID).Scan(&id)
	if err != nil {
		t.Fatalf("insert container project: %v", err)
	}
	return id
}

// TestListProjects_ExcludesContainers is the guard rail for the hidden-container
// design: a container row is storage for a lite atom, never a project. If this
// ever fails, containers are leaking into the student's project list — and by
// the same query shape, into teacher dashboards and cost aggregates.
func TestListProjects_ExcludesContainers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	containerID := insertContainerProject(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /projects = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	for _, p := range out.Projects {
		if p.ID == containerID {
			t.Fatalf("container project %s leaked into GET /projects", containerID)
		}
	}
}
