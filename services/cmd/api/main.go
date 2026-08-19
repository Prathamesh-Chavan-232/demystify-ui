package main

import (
	"log/slog"
	"os"

	"github.com/Prathamesh-Chavan-232/demystify-ui/services/internal/httpx"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(httpx.HealthCheck("API_PORT", "8080"))
	}
	if err := httpx.Serve("api", "API_PORT", "8080", nil); err != nil {
		slog.Error("server failed", "service", "api", "err", err)
		os.Exit(1)
	}
}
