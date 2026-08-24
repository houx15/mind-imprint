package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostFeedback(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)

	rr := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"text": "很好用"})
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(body)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth want 401 got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	empty, _ := json.Marshal(map[string]string{"text": "  "})
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(empty)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty want 400 got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/feedback", bytes.NewReader(body)), cookie))
	if rr.Code != http.StatusCreated {
		t.Fatalf("ok want 201 got %d (%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == "" {
		t.Fatalf("want non-empty id, got %q", resp.ID)
	}
}
