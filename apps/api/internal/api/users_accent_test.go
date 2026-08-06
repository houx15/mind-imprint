package api_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestPutUserAccent(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)

	// Unauthenticated → 401.
	body, _ := json.Marshal(map[string]string{"accent": "teal"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/v1/users/me/accent", bytes.NewReader(body)))
	if rr.Code != 401 {
		t.Fatalf("no cookie: want 401, got %d — %s", rr.Code, rr.Body.String())
	}

	// Known preset → 200 + echoes back.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/users/me/accent", bytes.NewReader(body)), cookie))
	if rr.Code != 200 {
		t.Fatalf("teal: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Accent string `json:"accent"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Accent != "teal" {
		t.Fatalf("want accent=teal echoed, got %q — %s", resp.Accent, rr.Body.String())
	}

	// Persisted: the users row now carries the new avatar_color.
	q := sqlc.New(pool)
	u, err := q.GetUserByID(t.Context(), SeedUserID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.AvatarColor != "teal" {
		t.Fatalf("avatar_color: want teal, got %q", u.AvatarColor)
	}

	// Unknown preset → 400.
	badBody, _ := json.Marshal(map[string]string{"accent": "chartreuse"})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/users/me/accent", bytes.NewReader(badBody)), cookie))
	if rr.Code != 400 {
		t.Fatalf("unknown accent: want 400, got %d — %s", rr.Code, rr.Body.String())
	}
}
