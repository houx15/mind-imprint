package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// helper: admin assigns teacherID to classID; returns the response recorder.
func assignTeacher(t *testing.T, h http.Handler, admin *http.Cookie, classID, teacherID string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"teacher_user_id":"` + teacherID + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/classes/"+classID+"/teachers", body)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAssignAndRemoveClassTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	// A class owned by teacher A (in the seed school).
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	classID := createClassViaAPI(t, h, signInAs(t, pool, aID), "TOK 12B")
	// Teacher B to be assigned.
	bID := createTeacher(t, pool, SeedSchoolID, "b@demo.local")

	rec := assignTeacher(t, h, admin, classID, bID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("assign: want 200, got %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Teachers []struct{ Email string } `json:"teachers"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Teachers) != 2 {
		t.Fatalf("after assign want 2 teachers, got %d", len(got.Teachers))
	}

	// Idempotent: re-assigning B is a no-op (still 2).
	rec = assignTeacher(t, h, admin, classID, bID.String())
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || len(got.Teachers) != 2 {
		t.Fatalf("re-assign should be idempotent, got %d / %d teachers", rec.Code, len(got.Teachers))
	}

	// Remove B → 204, back to 1 teacher.
	req := httptest.NewRequest("DELETE", "/api/v1/classes/"+classID+"/teachers/"+bID.String(), nil)
	req.AddCookie(admin)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, req)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("remove: want 204, got %d: %s", delRec.Code, delRec.Body)
	}
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil)
	getReq.AddCookie(admin)
	h.ServeHTTP(getRec, getReq)
	_ = json.Unmarshal(getRec.Body.Bytes(), &got)
	if len(got.Teachers) != 1 {
		t.Fatalf("after remove want 1 teacher, got %d", len(got.Teachers))
	}
}

func TestAssignTeacherRejectsBadTarget(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	classID := createClassViaAPI(t, h, signInAs(t, pool, aID), "C")

	// A random (non-existent) uuid is not a teacher in the school → 400.
	rec := assignTeacher(t, h, admin, classID, "00000000-0000-0000-0000-0000000000ff")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid teacher, got %d", rec.Code)
	}
}

func TestAssignTeacherCrossSchool404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool) // school = seed (A)

	// A class in another school B.
	otherSchool := seedSecondSchool(t, pool)
	bTeacher := createTeacher(t, pool, otherSchool, "bteach@demo.local")
	classB := createClassViaAPI(t, h, signInAs(t, pool, bTeacher), "B-class")
	someTeacher := createTeacher(t, pool, otherSchool, "x@demo.local")

	rec := assignTeacher(t, h, admin, classB, someTeacher.String())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin of A assigning into B's class: want 404, got %d", rec.Code)
	}
}

func TestAssignTeacherRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	teacher := signInAs(t, pool, aID)
	classID := createClassViaAPI(t, h, teacher, "C")

	rec := assignTeacher(t, h, teacher, classID, aID.String())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a teacher calling assign: want 403, got %d", rec.Code)
	}
}
