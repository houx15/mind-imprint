package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

// roleProbe is a trivial protected handler that 200s if reached.
func roleProbe(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireRoleForbidsWrongRole(t *testing.T) {
	pool := newAPITestPool(t)
	h := SessionAuthForTest(pool, RequireUser(RequireRole("admin")(http.HandlerFunc(roleProbe))))

	// Seeded student (Phoebe) must be forbidden from an admin-only route.
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/probe", nil), signInSeed(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}

	// Seeded admin passes.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/probe", nil), signInAdmin(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin got %d, want 200", rec.Code)
	}
}

func TestAssertAdminOfSchoolRejectsOtherSchool(t *testing.T) {
	pool := newAPITestPool(t)
	a := newTestAPI(pool) // existing helper that builds *API from the pool
	ctx := WithUser(context.Background(), User{
		ID: SeedAdminID, SchoolID: SeedSchoolID, Role: "admin",
	})
	if err := a.AssertAdminOfSchoolForTest(ctx, SeedSchoolID); err != nil {
		t.Fatalf("same school must pass: %v", err)
	}
	other := mustUUID("00000000-0000-0000-0000-0000000000ff")
	if err := a.AssertAdminOfSchoolForTest(ctx, other); err == nil {
		t.Fatal("other school must be rejected")
	}
}
