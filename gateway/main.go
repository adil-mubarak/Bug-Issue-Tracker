package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adil/bug-issue-tracker/pkg/auth"
	"github.com/adil/bug-issue-tracker/pkg/health"
	"github.com/adil/bug-issue-tracker/pkg/logging"
	"github.com/adil/bug-issue-tracker/pkg/ratelimit"
	"github.com/adil/bug-issue-tracker/pkg/requestid"
)

func main() {
	logging.Configure()
	secret := []byte(env("JWT_SECRET", ""))
	if len(secret) < 32 {
		slog.Error("JWT_SECRET must contain at least 32 characters")
		os.Exit(1)
	}
	bucket := ratelimit.New(120, 2)
	transport := &http.Transport{ResponseHeaderTimeout: 5 * time.Second}
	mux := http.NewServeMux()
	mux.Handle("/health", health.Handler(nil))
	mux.Handle("/ready", health.Handler(nil))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Accept both direct gateway paths (/auth/...) and paths forwarded by a
		// proxy that did not strip the frontend's /api prefix (/api/auth/...).
		r.URL.Path = normalizeAPIPath(r.URL.Path)
		if !bucket.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		var target string
		protected := true
		switch {
		case strings.HasPrefix(r.URL.Path, "/auth/"):
			target, protected = env("AUTH_SERVICE_URL", "http://auth:8081"), false
		case strings.HasPrefix(r.URL.Path, "/projects"):
			target = env("PROJECT_SERVICE_URL", "http://project:8082")
		case strings.HasPrefix(r.URL.Path, "/issues"):
			target = env("ISSUE_SERVICE_URL", "http://issue:8083")
		case strings.HasPrefix(r.URL.Path, "/activity"):
			target = env("ACTIVITY_SERVICE_URL", "http://activity:8084")
		default:
			http.NotFound(w, r)
			return
		}
		if protected {
			parts := strings.Fields(strings.TrimSpace(r.Header.Get("Authorization")))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, role, err := auth.VerifyToken(parts[1], secret)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			r.Header.Set("X-User-ID", strconv.FormatUint(userID, 10))
			r.Header.Set("X-User-Role", role)
		}
		destination, err := url.Parse(target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(destination)
		proxy.Transport = transport
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
			slog.Warn("upstream request failed", "error", err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: env("GATEWAY_ADDR", ":8080"), Handler: requestid.Middleware(mux), ReadHeaderTimeout: 5 * time.Second}
	slog.Info("gateway listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("gateway", "error", err)
		os.Exit(1)
	}
}

func normalizeAPIPath(path string) string {
	if strings.HasPrefix(path, "/api/") {
		return strings.TrimPrefix(path, "/api")
	}
	return path
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
