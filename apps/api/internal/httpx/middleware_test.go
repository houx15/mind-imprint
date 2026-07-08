package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	var seen string
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}), RequestID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(rec, req)

	if seen == "" {
		t.Fatal("request id not present in context")
	}
	if got := rec.Header().Get("X-Request-ID"); got != seen {
		t.Fatalf("X-Request-ID header = %q, context = %q", got, seen)
	}
}

func TestRecover(t *testing.T) {
	// Silence slog output during this test to keep output clean.
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), RequestID, Recover)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	// Must not propagate the panic.
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	env := decodeEnvelope(t, rec.Body.String())
	if env["code"] != "internal_error" {
		t.Fatalf("code = %v, want internal_error", env["code"])
	}
}

func TestCORSPreflight(t *testing.T) {
	allowed := "http://localhost:5173"
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), CORS([]string{allowed}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", allowed)
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Fatalf("Allow-Origin = %q, want %q", got, allowed)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
}

// TestCORSPreflightPatchDelete guards the regression where the allow-list omitted
// PATCH/DELETE, silently breaking card-open (PATCH), class-rename (PATCH), and
// student/teacher removal (DELETE) from the browser.
func TestCORSPreflightPatchDelete(t *testing.T) {
	allowed := "http://localhost:5173"
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), CORS([]string{allowed}))

	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/x", nil)
		req.Header.Set("Origin", allowed)
		req.Header.Set("Access-Control-Request-Method", method)
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
			t.Fatalf("%s preflight: Allow-Origin = %q, want %q", method, got, allowed)
		}
	}
}
