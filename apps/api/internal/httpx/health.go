package httpx

import (
	"context"
	"net/http"
	"time"
)

// Pinger is the minimal dependency Readyz needs — satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz is liveness: the process is up. Always 200 "ok".
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Readyz is readiness: dependencies are reachable. Pings the pool with a short
// timeout; 200 when ok, 503 envelope when not.
func Readyz(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := p.Ping(ctx); err != nil {
			WriteError(w, r, &APIError{
				Status:  http.StatusServiceUnavailable,
				Code:    "unavailable",
				Message: "依赖未就绪",
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
