package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		ready func() error
		want  int
	}{
		{name: "liveness", path: "/health", want: http.StatusOK},
		{name: "readiness", path: "/ready", want: http.StatusOK},
		{name: "dependency unavailable", path: "/ready", ready: func() error { return errors.New("database unavailable") }, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			Handler(tt.ready).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
		})
	}
}
