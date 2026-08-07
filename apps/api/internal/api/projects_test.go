package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestProjectsEndpoints(t *testing.T) {
	pool := newAPITestPool(t) // applies migrations incl. the 0018 demo seed
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()

	// 401 unauthenticated.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/projects", nil))
	if rr.Code != 401 {
		t.Fatalf("list no cookie: want 401, got %d", rr.Code)
	}

	cookie := signInSeed(t, pool) // Phoebe (SeedUserID = …0003)

	// list includes the seeded project + its derived lifecycle status. The seed
	// project has no proposal and no plan items → forming (BE5).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "0457 个人报告") {
		t.Fatalf("list: %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"forming"`) {
		t.Fatalf("list missing derived status=forming — %s", rr.Body.String())
	}

	// detail returns the lean workspace projection {id,title,qualification,proposal}.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/00000000-0000-0000-0000-000000000101", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("detail: %d — %s", rr.Code, rr.Body.String())
	}
	for _, want := range []string{
		`"id":"00000000-0000-0000-0000-000000000101"`,
		`"title":"To what extent is China making the world more environmentally sustainable?"`,
		`"qualification":"0457 个人报告"`,
		`"proposal":{"objective":"","reason":"","activities":"","resources":"","counterpoints":""}`,
		`"status":"forming"`,
	} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("detail missing %s — %s", want, rr.Body.String())
		}
	}

	// 404 for another user's project id (use a random non-owned uuid).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign project: want 404, got %d", rr.Code)
	}
}

// TestProjectStatus_FormingToWorking — filling any kick-off dimension flips the
// derived projection status from forming to working (BE5).
func TestProjectStatus_FormingToWorking(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101"

	// A blank project is forming.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if !strings.Contains(rr.Body.String(), `"status":"forming"`) {
		t.Fatalf("fresh project status = %s, want forming", rr.Body)
	}

	// Fill one proposal dimension.
	rrPut := httptest.NewRecorder()
	h.ServeHTTP(rrPut, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证国内新能源投资的可持续影响","reason":"","activities":"","resources":""}`)), cookie))
	if rrPut.Code != 200 {
		t.Fatalf("PUT proposal = %d — %s", rrPut.Code, rrPut.Body)
	}

	// Now working.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if !strings.Contains(rr2.Body.String(), `"status":"working"`) {
		t.Fatalf("project status after proposal = %s, want working", rr2.Body)
	}
}
