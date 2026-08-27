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
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// liteHandler builds an API whose seed SCHOOL is on the lite edition, plus a
// signed-in cookie. Shared by every lite test in this package.
func liteHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	return liteHandlerWithProvider(t, nil)
}

// liteHandlerWithProvider is liteHandler with a scripted model behind it —
// the coach-turn tests (Task 7) need one; every other lite test passes nil
// because no lite endpoint but the turn calls a model.
func liteHandlerWithProvider(t *testing.T, prov gateway.Provider) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool, Provider: prov,
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

// TestMeCarriesSchoolEdition — /auth/me must report which edition the
// signed-in user's SCHOOL is on.
//
// This is load-bearing, not cosmetic: it is the only thing either frontend
// can read to decide whether the student is standing in the right app. Both
// apps show the same auth screen and share one session cookie across
// *.uni-robot.cn, so after sign-in each one compares its own edition against
// this field and sends the student to the other host when they differ. If
// this field ever stops reflecting schools.edition, a lite student silently
// lands in the pro app — where every lite route 404s and nothing works.
//
// Asserted in BOTH directions on purpose: a handler that hardcoded either
// value would pass a one-sided test.
func TestMeCarriesSchoolEdition(t *testing.T) {
	t.Run("lite school reports lite", func(t *testing.T) {
		h, cookie, _, _ := liteHandler(t)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/auth/me", nil), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("lite GET /auth/me = %d, want 200; body=%s", rec.Code, rec.Body)
		}
		if got := meEdition(t, rec.Body.Bytes()); got != "lite" {
			t.Fatalf("lite school /auth/me edition = %q, want %q; body=%s", got, "lite", rec.Body)
		}
	})

	t.Run("pro school reports pro", func(t *testing.T) {
		pool := newAPITestPool(t)
		h := New(Deps{
			Queries: sqlc.New(pool), Pool: pool,
			ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
		}).Handler()
		cookie := signInSeed(t, pool)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/auth/me", nil), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("pro GET /auth/me = %d, want 200; body=%s", rec.Code, rec.Body)
		}
		if got := meEdition(t, rec.Body.Bytes()); got != "pro" {
			t.Fatalf("pro school /auth/me edition = %q, want %q; body=%s", got, "pro", rec.Body)
		}
	})
}

// meEdition digs school.edition out of a /auth/me body. Decoded rather than
// substring-matched so a field nested under the wrong object cannot pass.
func meEdition(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		User struct {
			School struct {
				Edition string `json:"edition"`
			} `json:"school"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode /auth/me body: %v — %s", err, body)
	}
	return payload.User.School.Edition
}
