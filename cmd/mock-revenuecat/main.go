// Command mock-revenuecat serves the stateful RevenueCat API v2 fake over HTTP.
//
// It exists so a container-based end-to-end run can drive the provider with a
// real terraform binary and no RevenueCat account. The Go tests mount the same
// handler in-process; this binary is only a different way in, so the two cannot
// drift apart.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/mockrevenuecat"
)

func main() {
	var (
		addr            = flag.String("addr", envOr("MOCK_ADDR", ":8080"), "address to listen on")
		apiKey          = flag.String("api-key", envOr("MOCK_API_KEY", ""), "the only bearer token to accept; empty accepts any non-empty token")
		pageSize        = flag.Int("page-size", envOrInt("MOCK_PAGE_SIZE", 50), "how many items a list response carries before paginating")
		rateLimit       = flag.Int("rate-limit-requests", envOrInt("MOCK_RATE_LIMIT_REQUESTS", 0), "answer this many requests with 429 before serving normally")
		seedProjectID   = flag.String("seed-project-id", envOr("MOCK_SEED_PROJECT_ID", "proj_mock"), "identifier of a project to pre-create")
		seedProjectName = flag.String("seed-project-name", envOr("MOCK_SEED_PROJECT_NAME", "Acme"), "name of the pre-created project")
	)
	flag.Parse()

	server := mockrevenuecat.New(mockrevenuecat.Options{
		APIKey:            *apiKey,
		PageSize:          *pageSize,
		RateLimitRequests: *rateLimit,
		SeedProjectID:     *seedProjectID,
		SeedProjectName:   *seedProjectName,
	})

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("mock RevenueCat API listening on %s", *addr)
	log.Printf("seeded project %q named %q", *seedProjectID, *seedProjectName)
	if *apiKey != "" {
		log.Printf("requiring the configured API key")
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("mock server: %v", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envOrInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		log.Printf("ignoring %s=%q: not an integer", name, value)
		return fallback
	}
	return parsed
}
