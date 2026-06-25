package api_test

import (
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
