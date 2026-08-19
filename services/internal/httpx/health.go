package httpx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HealthCheck probes this process's own /healthz and reports the result as an
// exit code. Distroless images carry no shell, curl, or wget, so the compose
// healthcheck invokes the binary itself: ["CMD", "/api", "-health"].
func HealthCheck(portEnv, fallbackPort string) int {
	port := os.Getenv(portEnv)
	if port == "" {
		port = fallbackPort
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
