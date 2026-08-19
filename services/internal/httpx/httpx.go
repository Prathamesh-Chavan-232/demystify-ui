// Package httpx holds the HTTP server setup shared by the demystify services.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Serve runs an HTTP server for the named service on the port named by portEnv,
// falling back to fallbackPort, and blocks until SIGINT or SIGTERM. A /healthz
// route is always registered: the compose healthchecks depend on it, and
// dependents gate on condition: service_healthy.
func Serve(name, portEnv, fallbackPort string, mux *http.ServeMux) error {
	if mux == nil {
		mux = http.NewServeMux()
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := os.Getenv(portEnv)
	if port == "" {
		port = fallbackPort
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "service", name, "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
