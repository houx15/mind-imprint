package api_test

// course_session_test.go — Course Runtime Slice 8: CourseSession snapshot
// persistence. POST get-or-creates the authed student's session; PUT snapshot-
// saves the whole blob; a second student never reads the first's session.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func postSession(t *testing.T, h http.Handler, cookie *http.Cookie, slug string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+slug+"/session", bytes.NewReader([]byte(`{}`))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST session: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Session map[string]any `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode session: %v — body %s", err, rec.Body)
	}
	return resp.Session
}

// TestCourseSessionGetOrCreateAndSave — create returns a fresh "created" session
// carrying the definition's course.id + the authed student; a PUT snapshot then
// a second POST returns the SAVED blob (get-or-create resumes, never re-mints).
func TestCourseSessionGetOrCreateAndSave(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	created := postSession(t, h, cookie, "def-course")
	if created["status"] != "created" {
		t.Fatalf("new session status = %v, want created", created["status"])
	}
	if created["courseId"] != "test-runtime-course" {
		t.Fatalf("session courseId = %v, want test-runtime-course", created["courseId"])
	}
	if created["studentId"] != SeedUserID.String() {
		t.Fatalf("session studentId = %v, want the authed user %s", created["studentId"], SeedUserID)
	}

	// Mutate the blob (advance status to in-progress) and snapshot-save.
	created["status"] = "in-progress"
	body, _ := json.Marshal(map[string]any{"session": created})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/def-course/session", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT session: want 200, got %d %s", rec.Code, rec.Body)
	}

	// A second get-or-create returns the SAVED (mutated) blob, not a new one.
	resumed := postSession(t, h, cookie, "def-course")
	if resumed["status"] != "in-progress" {
		t.Fatalf("resumed session status = %v, want in-progress (the saved blob)", resumed["status"])
	}
	if resumed["id"] != created["id"] {
		t.Fatalf("resumed session id = %v, want the same session %v (get-or-create must not re-mint)", resumed["id"], created["id"])
	}
}

// TestCourseSessionOwnerScoped — a second student's get-or-create returns THEIR
// OWN new session, never the first student's.
func TestCourseSessionOwnerScoped(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	first := postSession(t, h, signInSeed(t, pool), "def-course")

	// A different student in the same school.
	other := createStudent(t, pool, seedSchoolID(t, pool), "second-student@example.com")
	second := postSession(t, h, signInAs(t, pool, other), "def-course")

	if second["id"] == first["id"] {
		t.Fatalf("second student got the first's session id %v — sessions must be owner-scoped", first["id"])
	}
	if second["studentId"] != other.String() {
		t.Fatalf("second session studentId = %v, want the second student %s", second["studentId"], other)
	}
}

// seedSchoolID returns the seeded student's school id (for placing a second
// student in the same school without hardcoding the seed uuid).
func seedSchoolID(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT school_id FROM users WHERE id = $1`, SeedUserID).Scan(&id); err != nil {
		t.Fatalf("seedSchoolID: %v", err)
	}
	return id
}
