package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminListTeachers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	createTeacher(t, pool, SeedSchoolID, "t1@demo.local")
	createTeacher(t, pool, SeedSchoolID, "t2@demo.local")
	// A teacher in another school must NOT appear.
	otherSchool := seedSecondSchool(t, pool)
	createTeacher(t, pool, otherSchool, "other@demo.local")

	// Seed school also carries the D1 demo teacher (吴老师, migration 0029), so
	// the school now has 3 teachers total: t1, t2, and 吴老师.

	req := httptest.NewRequest("GET", "/api/v1/admin/teachers", nil)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Teachers []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"teachers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Teachers) != 3 {
		t.Fatalf("want 3 teachers in school, got %d", len(body.Teachers))
	}
	for _, te := range body.Teachers {
		if te.Email == "other@demo.local" {
			t.Fatal("leaked a teacher from another school")
		}
		if te.ID == "" || te.DisplayName == "" {
			t.Fatalf("incomplete teacher dto: %+v", te)
		}
	}
}

func TestAdminListTeachersRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	tid := createTeacher(t, pool, SeedSchoolID, "t@demo.local")
	teacher := signInAs(t, pool, tid)

	req := httptest.NewRequest("GET", "/api/v1/admin/teachers", nil)
	req.AddCookie(teacher)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for a teacher, got %d", rec.Code)
	}
}
