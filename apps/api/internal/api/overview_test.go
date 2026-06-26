package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminOverviewCountsScopedToSchool(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	// Seed school already has 1 student (Phoebe), 1 admin, 1 class.
	seedTaskFor(t, pool, SeedUserID) // 1 task, 1 active student

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/admin/overview", nil), signInAdmin(t, pool)))
	if rec.Code != http.StatusOK {
		t.Fatalf("overview got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Counts struct {
			Student       int `json:"student"`
			Class         int `json:"class"`
			Task          int `json:"task"`
			ActiveStudent int `json:"active_student"`
		} `json:"counts"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Counts.Student < 1 || resp.Counts.Class < 1 || resp.Counts.Task < 1 || resp.Counts.ActiveStudent < 1 {
		t.Fatalf("counts wrong: %+v", resp.Counts)
	}
}

func TestOverviewForbiddenToTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ov@demo.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/admin/overview", nil), teacher))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("teacher got %d, want 403", rec.Code)
	}
}
