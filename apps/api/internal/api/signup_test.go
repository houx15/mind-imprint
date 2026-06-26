package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestSignup(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	// Happy path: join the seeded class.
	body := `{"email":"alice@demo.local","password":"alice-pass-1","display_name":"Alice","join_code":"DEMO-0001"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body)))
	if rr.Code != 201 {
		t.Fatalf("signup: want 201, got %d — %s", rr.Code, rr.Body.String())
	}

	// Duplicate email → 409 email_taken.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body)))
	if rr.Code != 409 || !strings.Contains(rr.Body.String(), "email_taken") {
		t.Fatalf("dup email: want 409 email_taken, got %d — %s", rr.Code, rr.Body.String())
	}

	// Bad join code → 400 invalid_join_code.
	bad := `{"email":"bob@demo.local","password":"bob-pass-1","display_name":"Bob","join_code":"NOPE"}`
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(bad)))
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), "invalid_join_code") {
		t.Fatalf("bad code: want 400 invalid_join_code, got %d — %s", rr.Code, rr.Body.String())
	}

	// Short password → 400 validation_failed.
	short := `{"email":"c@demo.local","password":"x","display_name":"C","join_code":"DEMO-0001"}`
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(short)))
	if rr.Code != 400 {
		t.Fatalf("short pw: want 400, got %d — %s", rr.Code, rr.Body.String())
	}
}

func TestSignupWithTeacherInviteCreatesTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()

	// Admin mints an invite (no bound email).
	admin := signInAdmin(t, pool)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", strings.NewReader("{}")), admin)
	h.ServeHTTP(rec, req)
	var created struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &created)

	// Teacher signs up with it.
	body, _ := json.Marshal(map[string]any{
		"email": "tt@demo.local", "password": "password123",
		"display_name": "Teacher T", "join_code": created.Code,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("teacher signup got %d body=%s", rec.Code, rec.Body)
	}

	q := sqlc.New(pool)
	u, err := q.GetUserByEmail(context.Background(), "tt@demo.local")
	if err != nil || u.Role != "teacher" {
		t.Fatalf("expected teacher user, got role=%q err=%v", u.Role, err)
	}
	if u.SchoolID != SeedSchoolID {
		t.Fatalf("teacher school = %v, want seed", u.SchoolID)
	}
	// Invite is consumed → reusing it fails.
	rec = httptest.NewRecorder()
	body2, _ := json.Marshal(map[string]any{
		"email": "tt2@demo.local", "password": "password123",
		"display_name": "Teacher Two", "join_code": created.Code,
	})
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body2)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reused invite got %d, want 400", rec.Code)
	}
}

func TestSignupTeacherInviteEmailMismatch(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)
	body, _ := json.Marshal(map[string]any{"email": "bound@demo.local"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", bytes.NewReader(body)), admin))
	var created struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &created)

	// Sign up with a different email than the invite was bound to.
	su, _ := json.Marshal(map[string]any{
		"email": "someoneelse@demo.local", "password": "password123",
		"display_name": "X", "join_code": created.Code,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(su)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("email mismatch got %d, want 400", rec.Code)
	}
}
