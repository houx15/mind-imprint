package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestWeeklyReportRejectsForeignTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")

	intruder := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-intruder@demo.local"))
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), intruder)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404 — a foreign class must be existence-hidden", rec.Code)
	}
}

func TestWeeklyReportRejectsStudent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner2@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")

	student := signInSeed(t, pool)
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), student)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", rec.Code)
	}
}

func TestWeeklyReportShapeWithoutProse(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-shape@demo.local"))
	classID := createClassViaAPI(t, h, owner, "周报班")
	studentID := createStudent(t, pool, SeedSchoolID, "wk-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), owner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var got WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Stats) != 4 {
		t.Fatalf("stats = %d; want 4", len(got.Stats))
	}
	if got.ProseReady {
		t.Fatal("proseReady = true; nothing has been generated yet")
	}
	if got.Comment != nil {
		t.Fatalf("comment = %v; want null before generation", *got.Comment)
	}
	if got.ClassSize != 1 {
		t.Fatalf("classSize = %d; want 1", got.ClassSize)
	}
	if got.WeekLabel == "" {
		t.Fatal("weekLabel must always be present")
	}
}
