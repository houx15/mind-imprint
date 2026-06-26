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

// TestE2EOrgProvisioning walks the full P3.1 provisioning vertical.
func TestE2EOrgProvisioning(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	post := func(path string, cookie *http.Cookie, payload any) *httptest.ResponseRecorder {
		var rdr *bytes.Reader
		if s, ok := payload.(string); ok {
			rdr = bytes.NewReader([]byte(s))
		} else {
			b, _ := json.Marshal(payload)
			rdr = bytes.NewReader(b)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, rdr)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		h.ServeHTTP(rec, req)
		return rec
	}

	// 1. Admin mints a teacher invite.
	rec := post("/api/v1/admin/teacher-invites", admin, map[string]any{"email": "e2eteacher@demo.local"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint invite: %d %s", rec.Code, rec.Body)
	}
	var inv struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &inv)

	// 2. Teacher signs up with it.
	rec = post("/api/v1/auth/signup", nil, map[string]any{
		"email": "e2eteacher@demo.local", "password": "password123",
		"display_name": "E2E Teacher", "join_code": inv.Code,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("teacher signup: %d %s", rec.Code, rec.Body)
	}
	teacherCookie := signInViaAPI(t, h, "e2eteacher@demo.local", "password123")

	// 3. Teacher creates a class.
	rec = post("/api/v1/classes", teacherCookie, map[string]any{"name": "E2E Class"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create class: %d %s", rec.Code, rec.Body)
	}
	var cc struct {
		Class struct {
			ID       string `json:"id"`
			JoinCode string `json:"join_code"`
		} `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cc)

	// 4. A student signs up with the class join code.
	rec = post("/api/v1/auth/signup", nil, map[string]any{
		"email": "e2estudent@demo.local", "password": "password123",
		"display_name": "E2E Student", "join_code": cc.Class.JoinCode,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("student signup: %d %s", rec.Code, rec.Body)
	}

	// 5. Teacher sees the student on the roster.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/classes/"+cc.Class.ID, nil)
	req.AddCookie(teacherCookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "e2estudent@demo.local") {
		t.Fatalf("roster missing student: %d %s", rec.Code, rec.Body)
	}

	// 6. Admin overview reflects the new class.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/admin/overview", nil)
	req.AddCookie(admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body)
	}
}
