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

func TestClassRosterShowsAggregateSignals(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacherID := createTeacher(t, pool, SeedSchoolID, "rt@demo.local")
	teacher := signInAs(t, pool, teacherID)

	// Create a class, then enroll the seeded student (Phoebe) into it directly.
	classID := createClassViaAPI(t, h, teacher, "Roster Class")
	enrollStudent(t, pool, SeedUserID, classID)
	// Give Phoebe one task so a count is non-zero.
	seedTaskFor(t, pool, SeedUserID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("roster got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Roster []struct {
			Email     string `json:"email"`
			TaskCount int    `json:"task_count"`
		} `json:"roster"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Roster) != 1 || resp.Roster[0].TaskCount < 1 {
		t.Fatalf("unexpected roster: %+v", resp.Roster)
	}
	// Roster must NOT leak any contents field.
	if bytes.Contains(rec.Body.Bytes(), []byte("narrative")) {
		t.Fatal("roster leaked evaluation contents")
	}
}

func TestRosterDeniedToOtherSchoolTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Owned")

	// A teacher in a different school must get 404 (existence hidden).
	otherSchool := seedSecondSchool(t, pool)
	stranger := signInAs(t, pool, createTeacher(t, pool, otherSchool, "stranger@other.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil), stranger))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-school teacher got %d, want 404", rec.Code)
	}
}

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

func TestPatchClassRenameAndRegenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pt@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Old Name")

	body, _ := json.Marshal(map[string]any{"name": "New Name", "regenerate_join_code": true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/classes/"+classID, bytes.NewReader(body)), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Class struct {
			Name     string `json:"name"`
			JoinCode string `json:"join_code"`
		} `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Class.Name != "New Name" || resp.Class.JoinCode == "" {
		t.Fatalf("rename/regenerate failed: %+v", resp.Class)
	}
}

func TestRemoveStudentFromClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "rm@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "RM Class")
	enrollStudent(t, pool, SeedUserID, classID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("DELETE", "/api/v1/classes/"+classID+"/enrollments/"+SeedUserID.String(), nil), teacher))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove got %d", rec.Code)
	}
	// The student account still exists (only the enrollment was removed).
	if _, err := mustNewQueries(pool).GetUserByID(context.Background(), SeedUserID); err != nil {
		t.Fatalf("student account must survive: %v", err)
	}
}

func TestGetClassIncludesTeachers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	// A teacher creates a class → they are enrolled as its teacher.
	tid := createTeacher(t, pool, SeedSchoolID, "teach@demo.local")
	teacher := signInAs(t, pool, tid)
	classID := createClassViaAPI(t, h, teacher, "TOK 11A")

	req := httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil)
	req.AddCookie(teacher)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Teachers []struct {
			ID, DisplayName, Email string
		} `json:"teachers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Teachers) != 1 || body.Teachers[0].Email != "teach@demo.local" {
		t.Fatalf("want the creating teacher in teachers[], got %+v", body.Teachers)
	}
}
