package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
)

func TestTeacherCreatesAndListsOwnClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacherID := createTeacher(t, pool, SeedSchoolID, "tc@demo.local")
	teacher := signInAs(t, pool, teacherID)

	body, _ := json.Marshal(map[string]any{"name": "Block 3 History"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), teacher))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create got %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Class struct {
			ID       string `json:"id"`
			JoinCode string `json:"join_code"`
		} `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Class.JoinCode == "" {
		t.Fatal("class missing join_code")
	}
	// Creator is enrolled as teacher.
	q := mustNewQueries(pool)
	enr, err := q.GetEnrollment(context.Background(), GetEnrollmentParamsForTest(teacherID, created.Class.ID))
	if err != nil || enr.RoleInClass != "teacher" {
		t.Fatalf("creator not enrolled as teacher: %v", err)
	}

	// List returns it.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes", nil), teacher))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("Block 3 History")) {
		t.Fatalf("list got %d body=%s", rec.Code, rec.Body)
	}
}

func TestStudentCannotCreateClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader([]byte(`{"name":"x"}`))), signInSeed(t, pool)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}
}

func TestAdminCreatesClassForTeacherInSchool(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)
	teacherID := createTeacher(t, pool, SeedSchoolID, "assigned@demo.local")

	body, _ := json.Marshal(map[string]any{"name": "Admin Class", "teacher_user_id": teacherID.String()})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), admin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create got %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Class struct {
			ID string `json:"id"`
		} `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Class.ID == "" {
		t.Fatal("response missing class id")
	}
	// The ASSIGNED teacher (not the admin) must be enrolled as teacher.
	q := mustNewQueries(pool)
	enr, err := q.GetEnrollment(context.Background(), GetEnrollmentParamsForTest(teacherID, created.Class.ID))
	if err != nil || enr.RoleInClass != "teacher" {
		t.Fatalf("assigned teacher not enrolled as teacher: err=%v role=%q", err, enr.RoleInClass)
	}
}

func TestAdminCannotAssignForeignTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	otherSchool := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO schools (id, name) VALUES ($1, $2)`, otherSchool, "Other School"); err != nil {
		t.Fatalf("seed second school: %v", err)
	}
	foreignTeacher := createTeacher(t, pool, otherSchool, "foreign@other.local")

	body, _ := json.Marshal(map[string]any{"name": "Cross School Class", "teacher_user_id": foreignTeacher.String()})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), admin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-school assign got %d, want 400; body=%s", rec.Code, rec.Body)
	}
}
