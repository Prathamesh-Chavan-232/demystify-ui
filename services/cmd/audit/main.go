package main

import (
	"log/slog"
	"os"

	"github.com/Prathamesh-Chavan-232/demystify-ui/services/internal/httpx"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(httpx.HealthCheck("AUDIT_PORT", "8082"))
	}
	if err := httpx.Serve("audit", "AUDIT_PORT", "8082", nil); err != nil {
		slog.Error("server failed", "service", "audit", "err", err)
		os.Exit(1)
	}
}
