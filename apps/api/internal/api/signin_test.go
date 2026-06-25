package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestSigninSignout(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	// Seeded Phoebe (migration 0004) can sign in.
	creds := `{"email":"phoebe@demo.mindimprint.local","password":"phoebe-dev-pass"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin", strings.NewReader(creds)))
	if rr.Code != 200 {
		t.Fatalf("signin: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"display_name":"Phoebe"`) {
		t.Fatalf("signin body missing user: %s", rr.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "mk_session" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("signin did not set mk_session cookie")
	}

	// Wrong password → 401.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin",
		strings.NewReader(`{"email":"phoebe@demo.mindimprint.local","password":"nope"}`)))
	if rr.Code != 401 || !strings.Contains(rr.Body.String(), "invalid_credentials") {
		t.Fatalf("bad pw: want 401 invalid_credentials, got %d — %s", rr.Code, rr.Body.String())
	}

	// Unknown email → 401 (same code, no enumeration).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin",
		strings.NewReader(`{"email":"ghost@demo.local","password":"whatever1"}`)))
	if rr.Code != 401 {
		t.Fatalf("unknown email: want 401, got %d", rr.Code)
	}

	// Signout with the cookie → 204 and the session is revoked.
	req := httptest.NewRequest("POST", "/api/v1/auth/signout", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Fatalf("signout: want 204, got %d — %s", rr.Code, rr.Body.String())
	}
	q := sqlc.New(pool)
	if _, err := q.GetSessionWithUserByHash(context.Background(), auth.HashToken(cookie.Value)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("session not revoked after signout: %v", err)
	}
}
