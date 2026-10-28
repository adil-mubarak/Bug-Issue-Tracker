package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adil/bug-issue-tracker/pkg/health"
	"github.com/adil/bug-issue-tracker/pkg/logging"
	"github.com/adil/bug-issue-tracker/pkg/lru"
	"github.com/adil/bug-issue-tracker/pkg/mysqltls"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
)

func main() {
	logging.Configure()
	if err := mysqltls.RegisterFromEnv(); err != nil {
		slog.Error("configure database TLS", "error", err)
		os.Exit(1)
	}
	db, err := sql.Open("mysql", env("ISSUE_DATABASE_URL", ""))
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	nc, err := nats.Connect(env("NATS_URL", "nats://nats:4222"), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		slog.Warn("event broker unavailable; events will be skipped", "error", err)
	}
	if nc != nil {
		defer nc.Close()
	}
	h := &issueHandler{db: db, projectURL: env("PROJECT_SERVICE_URL", "http://project:8082"), client: &http.Client{Timeout: 3 * time.Second}, events: nc, memberships: lru.New[string, membershipEntry](2048)}
	mux := newIssueMux(h)
	mux.Handle("/health", health.Handler(nil))
	mux.Handle("/ready", health.Handler(func() error { return db.Ping() }))
	s := &http.Server{Addr: env("ISSUE_ADDR", ":8083"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("issue service listening", "address", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "error", err)
			os.Exit(1)
		}
	}()
	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-stop.Done()
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_ = s.Shutdown(ctx)
}

func newIssueMux(h *issueHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/issues", h)
	mux.Handle("/issues/", h)
	mux.Handle("/projects/", h)
	return mux
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
