package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminCreatesAndListsTeacherInvite(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	// Create.
	body, _ := json.Marshal(map[string]any{"email": "newteacher@demo.local"})
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", bytes.NewReader(body)), admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create got %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expires_at"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if !strings.HasPrefix(created.Code, "T-") || created.ExpiresAt == "" {
		t.Fatalf("bad create payload: %+v", created)
	}

	// List shows it.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/admin/teacher-invites", nil), admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), created.Code) {
		t.Fatalf("list got %d body=%s", rec.Code, rec.Body)
	}
}

func TestTeacherInviteRequiresAuth(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", strings.NewReader("{}"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated got %d, want 401", rec.Code)
	}
}

func TestTeacherInviteRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", strings.NewReader("{}")), signInSeed(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}
}
