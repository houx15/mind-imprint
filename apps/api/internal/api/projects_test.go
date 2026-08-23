package api_test

import (
	"context"
	"encoding/json"
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
	// Task 2: list items carry cover + coverUrl. The seeded project has no
	// cover set (nil column) → both surface as "" (OSS is also nil in tests).
	if !strings.Contains(rr.Body.String(), `"cover":""`) || !strings.Contains(rr.Body.String(), `"coverUrl":""`) {
		t.Fatalf("list missing cover/coverUrl fields — %s", rr.Body.String())
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

// TestProjectsList_CoverSurfacesInList — an "img:" cover set on a project row
// surfaces as-is in the list DTO's cover field. coverUrl stays "" here since
// the test Deps carry no OSS client (resolveCoverURL's nil-OSS guard) — the
// live-signed-URL path is exercised by resolveCoverURL's own unit coverage.
func TestProjectsList_CoverSurfacesInList(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	if _, err := pool.Exec(context.Background(),
		`UPDATE project SET cover = 'img:1' WHERE id = '00000000-0000-0000-0000-000000000101'`); err != nil {
		t.Fatalf("seed cover: %v", err)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("list: %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"cover":"img:1"`) {
		t.Fatalf("list missing cover=img:1 — %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"coverUrl":""`) {
		t.Fatalf("list coverUrl should be empty with no OSS configured — %s", rr.Body.String())
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

// TestProjectsList_DemoAppendedForEveryUser — the world-readable demo project
// (…0200, seeded is_demo by migration 0082, owner Phoebe/SeedUserID) shows up in
// EVERY user's list, pinned last and marked isDemo (guided-tour P5):
//   - a NON-owner sees it appended as the LAST entry with isDemo:true, while
//     their own project stays isDemo:false;
//   - the OWNER (Phoebe) sees it exactly ONCE (deduped, not duplicated) with
//     isDemo:true.
func TestProjectsList_DemoAppendedForEveryUser(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	q := sqlc.New(pool)
	ctx := context.Background()

	type item struct {
		ID     string `json:"id"`
		IsDemo bool   `json:"isDemo"`
	}

	// --- Non-owner: a fresh student with one project of their own. ---
	otherID := createStudent(t, pool, SeedSchoolID, "demo-list-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)
	if _, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: otherID, Qualification: "IB", Title: "非演示项目", BoardCfgVer: 1,
	}); err != nil {
		t.Fatalf("create non-owner project: %v", err)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("non-owner list: %d — %s", rr.Code, rr.Body.String())
	}
	var otherBody struct {
		Projects []item `json:"projects"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &otherBody); err != nil {
		t.Fatalf("decode non-owner list: %v — %s", err, rr.Body.String())
	}
	if len(otherBody.Projects) < 2 {
		t.Fatalf("non-owner list want >=2 (own + demo), got %d — %s", len(otherBody.Projects), rr.Body.String())
	}
	// Demo is the LAST entry, isDemo:true.
	last := otherBody.Projects[len(otherBody.Projects)-1]
	if last.ID != demoProjectID || !last.IsDemo {
		t.Fatalf("non-owner: last entry = %+v, want demo %s isDemo:true", last, demoProjectID)
	}
	// Every non-demo entry has isDemo:false, and the demo appears exactly once.
	demoCount := 0
	for _, p := range otherBody.Projects {
		if p.ID == demoProjectID {
			demoCount++
			if !p.IsDemo {
				t.Fatalf("non-owner: demo entry isDemo=false — %+v", p)
			}
		} else if p.IsDemo {
			t.Fatalf("non-owner: non-demo entry marked isDemo:true — %+v", p)
		}
	}
	if demoCount != 1 {
		t.Fatalf("non-owner: demo appears %d times, want exactly 1 — %s", demoCount, rr.Body.String())
	}

	// --- Owner (Phoebe) sees the demo exactly once (deduped), marked isDemo. ---
	ownerCookie := signInSeed(t, pool)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), ownerCookie))
	if rr.Code != 200 {
		t.Fatalf("owner list: %d — %s", rr.Code, rr.Body.String())
	}
	var ownerBody struct {
		Projects []item `json:"projects"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ownerBody); err != nil {
		t.Fatalf("decode owner list: %v — %s", err, rr.Body.String())
	}
	ownerDemoCount := 0
	for _, p := range ownerBody.Projects {
		if p.ID == demoProjectID {
			ownerDemoCount++
			if !p.IsDemo {
				t.Fatalf("owner: demo entry isDemo=false — %+v", p)
			}
		}
	}
	if ownerDemoCount != 1 {
		t.Fatalf("owner: demo appears %d times, want exactly 1 (deduped) — %s", ownerDemoCount, rr.Body.String())
	}
}
