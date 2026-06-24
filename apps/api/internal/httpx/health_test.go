package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	Healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name       string
		pinger     Pinger
		wantStatus int
	}{
		{"healthy → 200", stubPinger{nil}, http.StatusOK},
		{"db down → 503", stubPinger{errors.New("connection refused")}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			Readyz(tt.pinger).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusServiceUnavailable {
				env := decodeEnvelope(t, rec.Body.String())
				if env["code"] != "unavailable" {
					t.Fatalf("code = %v, want unavailable", env["code"])
				}
			}
		})
	}
}
