package main

import (
	"log/slog"
	"os"

	"github.com/Prathamesh-Chavan-232/demystify-ui/services/internal/httpx"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(httpx.HealthCheck("PALETTE_PORT", "8081"))
	}
	if err := httpx.Serve("palette", "PALETTE_PORT", "8081", nil); err != nil {
		slog.Error("server failed", "service", "palette", "err", err)
		os.Exit(1)
	}
}
