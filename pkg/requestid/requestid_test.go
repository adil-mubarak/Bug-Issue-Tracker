package requestid

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareValidatesAndPropagates(t *testing.T) {
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) == "" {
			t.Error("missing request ID")
		}
	}))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "test-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("X-Request-ID") != "test-123" {
		t.Fatal("valid ID not preserved")
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "bad id")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("X-Request-ID"); got == "bad id" || got == "" {
		t.Fatal("invalid ID not replaced")
	}
}
