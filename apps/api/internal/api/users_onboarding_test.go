package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestPutUserOnboarding(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)

	// Unauthenticated → 401.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/v1/users/me/onboarding", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	// Before: onboarded_at is NULL.
	q := sqlc.New(pool)
	before, err := q.GetUserByID(t.Context(), SeedUserID)
	if err != nil {
		t.Fatal(err)
	}
	if before.OnboardedAt.Valid {
		t.Fatalf("seed user should start un-onboarded")
	}

	// Authenticated → 200 and stamps the column.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/users/me/onboarding", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("stamp: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	after, err := q.GetUserByID(t.Context(), SeedUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.OnboardedAt.Valid {
		t.Fatalf("onboarded_at should be set after PUT")
	}
}
