package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adil/bug-issue-tracker/pkg/auth"
	"github.com/adil/bug-issue-tracker/pkg/ratelimit"
)

func TestNormalizeAPIPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "frontend api prefix", path: "/api/auth/login", want: "/auth/login"},
		{name: "unprefixed gateway route", path: "/auth/login", want: "/auth/login"},
		{name: "api prefix is segment-aware", path: "/apix/auth/login", want: "/apix/auth/login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAPIPath(tt.path); got != tt.want {
				t.Errorf("normalizeAPIPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGatewayAuthenticationAndRouting(t *testing.T) {
	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		if r.URL.Path != "/projects" {
			t.Errorf("upstream path=%q", r.URL.Path)
		}
		if r.Header.Get("X-User-ID") != "55" || r.Header.Get("X-User-Role") != "member" {
			t.Errorf("identity headers missing: %v", r.Header)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	secret := []byte("gateway-test-secret-at-least-32-bytes")
	handler := newGatewayHandler(secret, map[string]string{"projects": upstream.URL}, ratelimit.New(10, 1), &http.Transport{ResponseHeaderTimeout: time.Second})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/projects", nil)
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || upstreamHits != 0 {
		t.Fatalf("unauthorized status=%d upstreamHits=%d", w.Code, upstreamHits)
	}

	token, err := auth.IssueToken(55, "member", secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/projects", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Request-ID", "gateway-test-id")
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK || upstreamHits != 1 {
		t.Fatalf("authorized status=%d upstreamHits=%d body=%s", w.Code, upstreamHits, w.Body.String())
	}
	if w.Header().Get("X-Request-ID") != "gateway-test-id" {
		t.Fatalf("request ID not returned: %q", w.Header().Get("X-Request-ID"))
	}
}

func TestGatewayForwardsPublicAuthAndRejectsUnknownRoute(t *testing.T) {
	var called bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/auth/login" {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	handler := newGatewayHandler([]byte("gateway-test-secret-at-least-32-bytes"), map[string]string{"auth": upstream.URL}, ratelimit.New(10, 1), http.DefaultTransport)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))
	if w.Code != http.StatusOK || !called {
		t.Fatalf("public auth response %d called=%v", w.Code, called)
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects-evil", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown route status=%d", w.Code)
	}
}

func TestGatewayProtectsAndRoutesUsernameAutocomplete(t *testing.T) {
	secret := []byte("gateway-test-secret-at-least-32-bytes")
	var seenUser, seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, seenPath = r.Header.Get("X-User-ID"), r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer upstream.Close()
	handler := newGatewayHandler(secret, map[string]string{"auth": upstream.URL}, ratelimit.New(10, 1), http.DefaultTransport)
	token, err := auth.IssueToken(72, "member", secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/autocomplete?q=ab", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK || seenPath != "/users/autocomplete" || seenUser != "72" {
		t.Fatalf("status=%d upstream=(%q,%q)", w.Code, seenPath, seenUser)
	}
}

func TestGatewayReadinessChecksEveryUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()
	if err := checkUpstreams(map[string]string{"auth": upstream.URL, "issue": upstream.URL}, http.DefaultTransport); err != nil {
		t.Fatal(err)
	}
	if err := checkUpstreams(map[string]string{"missing": "http://127.0.0.1:1"}, http.DefaultTransport); err == nil {
		t.Fatal("expected unavailable upstream error")
	}
}
