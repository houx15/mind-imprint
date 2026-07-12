package api_test

// projectcards_test.go — Task 4: POST /api/v1/projects/{id}/cards/{cid}/activate
// and .../skip, the project-scoped card lifecycle endpoints that sit beside
// postProjectTurn's card surfacing (Task 5c-2). Uses the real testcontainers
// Postgres + the seeded demo project (00000000-0000-0000-0000-000000000101,
// owned by Phoebe) so loadOwnedProjectCard's membership scan exercises real
// rows, not a fixture.

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// createProjectCardForTest inserts a card_instance scoped to the seeded demo
// project (…0101) / task (…0100) via CreateProjectCardInstance, and returns
// its id as a string for building request paths.
func createProjectCardForTest(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	q := sqlc.New(pool)
	ci, err := q.CreateProjectCardInstance(t.Context(), sqlc.CreateProjectCardInstanceParams{
		TaskID:    mustUUID("00000000-0000-0000-0000-000000000100"),
		ProjectID: pgtype.UUID{Bytes: mustUUID("00000000-0000-0000-0000-000000000101"), Valid: true},
		CardID:    "sift_craap",
		Status:    "proposed",
	})
	if err != nil {
		t.Fatalf("createProjectCardForTest: %v", err)
	}
	return ci.ID.String()
}

func TestProjectCardActivateSkip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	cid := createProjectCardForTest(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/activate", nil), cookie))
	if rr.Code != 204 {
		t.Fatalf("activate: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/skip", strings.NewReader(`{"event_trace":[{"kind":"skip","at":"2026-07-12T00:00:00Z"}]}`)), cookie))
	if rr.Code != 204 {
		t.Fatalf("skip: %d — %s", rr.Code, rr.Body.String())
	}

	// 404 for a random (non-project) card id.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/"+uuid.NewString()+"/activate", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign card: %d", rr.Code)
	}
}
