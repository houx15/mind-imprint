package api_test

// project_rename_test.go — PATCH /api/v1/projects/{id}: a student renames their
// own project. Ownership is hidden as 404; an empty title is rejected.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestRenameProject_UpdatesTitle(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+materialsTestProjectID,
		strings.NewReader(`{"title":"  我的新项目名  "}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: %d — %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "我的新项目名") {
		t.Fatalf("rename body missing new title: %s", rec.Body.String())
	}

	// The GET projection now reflects the trimmed title.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+materialsTestProjectID, nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get project: %d — %s", rec.Code, rec.Body.String())
	}
	var proj struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if proj.Title != "我的新项目名" {
		t.Fatalf("project.title = %q, want trimmed %q", proj.Title, "我的新项目名")
	}
}

func TestRenameProject_RejectsEmptyTitle(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+materialsTestProjectID,
		strings.NewReader(`{"title":"   "}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty rename = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRenameProject_ForeignProject404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "rename-other@demo.local"))

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+materialsTestProjectID,
		strings.NewReader(`{"title":"越界改名"}`)), other)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign rename = %d, want 404 (ownership hidden); body=%s", rec.Code, rec.Body.String())
	}
}
