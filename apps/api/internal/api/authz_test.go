package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// roleProbe is a trivial protected handler that 200s if reached.
func roleProbe(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireRoleForbidsWrongRole(t *testing.T) {
	pool := newAPITestPool(t)
	h := SessionAuthForTest(pool, RequireUser(RequireRole("admin")(http.HandlerFunc(roleProbe))))

	// Seeded student (Phoebe) must be forbidden from an admin-only route.
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/probe", nil), signInSeed(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}

	// Seeded admin passes.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/probe", nil), signInAdmin(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin got %d, want 200", rec.Code)
	}
}

func TestAssertAdminOfSchoolRejectsOtherSchool(t *testing.T) {
	pool := newAPITestPool(t)
	a := newTestAPI(pool) // existing helper that builds *API from the pool
	ctx := WithUser(context.Background(), User{
		ID: SeedAdminID, SchoolID: SeedSchoolID, Role: "admin",
	})
	if err := a.AssertAdminOfSchoolForTest(ctx, SeedSchoolID); err != nil {
		t.Fatalf("same school must pass: %v", err)
	}
	other := mustUUID("00000000-0000-0000-0000-0000000000ff")
	if err := a.AssertAdminOfSchoolForTest(ctx, other); err == nil {
		t.Fatal("other school must be rejected")
	}
}

func TestAssertTeacherOwnsClass(t *testing.T) {
	pool := newAPITestPool(t)
	a := newTestAPI(pool)
	q := sqlc.New(pool)
	ctx := context.Background()

	// Create a teacher in the seeded school.
	teacherID := createTeacher(t, pool, SeedSchoolID, fmt.Sprintf("teacher-%s@test.local", uuid.New()))

	// Create a fresh class in the seeded school via raw SQL (no CreateClass sqlc query exists).
	classID := uuid.New()
	joinCode := fmt.Sprintf("TC-%s", uuid.New().String()[:8])
	if _, err := pool.Exec(ctx,
		`INSERT INTO classes (id, school_id, name, join_code) VALUES ($1, $2, $3, $4)`,
		classID, SeedSchoolID, "Test Class", joinCode,
	); err != nil {
		t.Fatalf("insert class: %v", err)
	}

	// Enroll the teacher in that class.
	if _, err := q.CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{
		UserID:      teacherID,
		ClassID:     classID,
		RoleInClass: "teacher",
	}); err != nil {
		t.Fatalf("create enrollment: %v", err)
	}

	// Case 1: teacher T owns the class → passes (nil error).
	teacherCtx := WithUser(ctx, User{ID: teacherID, SchoolID: SeedSchoolID, Role: "teacher"})
	if err := a.AssertTeacherOwnsClassForTest(teacherCtx, classID); err != nil {
		t.Fatalf("teacher should pass: %v", err)
	}

	// Case 2: seeded student (role=student) → error (not-found / not-owned).
	studentCtx := WithUser(ctx, User{ID: SeedUserID, SchoolID: SeedSchoolID, Role: "student"})
	if err := a.AssertTeacherOwnsClassForTest(studentCtx, classID); err == nil {
		t.Fatal("student must be rejected")
	}

	// Case 3: admin of the SAME school (SeedAdminID) → passes.
	adminCtx := WithUser(ctx, User{ID: SeedAdminID, SchoolID: SeedSchoolID, Role: "admin"})
	if err := a.AssertTeacherOwnsClassForTest(adminCtx, classID); err != nil {
		t.Fatalf("same-school admin should pass: %v", err)
	}

	// Case 4: admin of a DIFFERENT school → error.
	otherSchoolID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO schools (id, name) VALUES ($1, $2)`,
		otherSchoolID, "Other School",
	); err != nil {
		t.Fatalf("insert other school: %v", err)
	}
	otherAdminID := createTeacher(t, pool, otherSchoolID, fmt.Sprintf("admin-%s@other.local", uuid.New()))
	if _, err := pool.Exec(ctx, `UPDATE users SET role='admin' WHERE id=$1`, otherAdminID); err != nil {
		t.Fatalf("set role admin: %v", err)
	}
	otherAdminCtx := WithUser(ctx, User{ID: otherAdminID, SchoolID: otherSchoolID, Role: "admin"})
	if err := a.AssertTeacherOwnsClassForTest(otherAdminCtx, classID); err == nil {
		t.Fatal("admin of different school must be rejected")
	}

	// Case 5: non-existent class id with a valid user → error.
	if err := a.AssertTeacherOwnsClassForTest(teacherCtx, uuid.New()); err == nil {
		t.Fatal("non-existent class must be rejected")
	}
}
