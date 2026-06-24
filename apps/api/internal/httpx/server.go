package httpx

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mindimprint/api/internal/config"
)

// NewServer builds the fully-wired HTTP server: ServeMux with health routes,
// the middleware chain, and a ReadHeaderTimeout to blunt slow-loris attacks.
func NewServer(cfg config.Config, p Pinger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Healthz)
	mux.Handle("GET /readyz", Readyz(p))

	handler := chain(mux, RequestID, Recover, Logger, CORS(cfg.CORSOrigins))

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// RunServer starts srv and blocks until SIGINT/SIGTERM, then shuts down
// gracefully (stop accepting, drain in-flight) and runs onShutdown (e.g. closing
// the pool) within a bounded window.
func RunServer(srv *http.Server, onShutdown func(context.Context)) error {
	errc := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errc <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errc:
		return err
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(ctx)
		if onShutdown != nil {
			onShutdown(ctx)
		}
		return shutdownErr
	}
}
