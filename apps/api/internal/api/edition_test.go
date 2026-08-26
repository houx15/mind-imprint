package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// liteHandler builds an API whose seed SCHOOL is on the lite edition, plus a
// signed-in cookie. Shared by every lite test in this package.
func liteHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatalf("set school edition lite: %v", err)
	}
	return h, signInSeed(t, pool), q, pool
}

// TestEditionGate_LiteSchoolCannotReachProjects — a lite student has no
// projects; the pro surface simply is not there for them. 404, never 403.
func TestEditionGate_LiteSchoolCannotReachProjects(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("lite school GET /projects = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestEditionGate_ProSchoolKeepsProjects — the default edition is 'pro', so
// every existing school and every existing test keeps working untouched.
func TestEditionGate_ProSchoolKeepsProjects(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("pro school GET /projects = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}
