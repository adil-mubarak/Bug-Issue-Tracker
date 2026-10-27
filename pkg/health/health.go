// Package health provides the standard liveness and readiness handlers used by
// every HTTP service.
package health

import (
	"encoding/json"
	"net/http"
)

// Handler returns an HTTP handler with /health and /ready endpoints. The
// readiness callback should check dependencies such as a database connection.
// A nil callback means the service is ready.
func Handler(ready func() error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(w, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if ready != nil {
			if err := ready(); err != nil {
				writeStatus(w, http.StatusServiceUnavailable, "not_ready")
				return
			}
		}
		writeStatus(w, http.StatusOK, "ready")
	})
	return mux
}

func writeStatus(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": message})
}
