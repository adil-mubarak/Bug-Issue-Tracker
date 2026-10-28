package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adil/bug-issue-tracker/pkg/health"
	"github.com/adil/bug-issue-tracker/pkg/logging"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
)

type event struct {
	EventID   string `json:"event_id"`
	RequestID string `json:"request_id,omitempty"`
	IssueID   uint64 `json:"issue_id"`
	ProjectID uint64 `json:"project_id"`
	ActorID   uint64 `json:"actor_id"`
	EventType string `json:"event_type"`
	Status    string `json:"status"`
}

func main() {
	logging.Configure()
	db, err := sql.Open("mysql", env("ACTIVITY_DATABASE_URL", ""))
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	nc, err := nats.Connect(env("NATS_URL", "nats://nats:4222"), nats.MaxReconnects(-1))
	if err != nil {
		slog.Error("connect event broker", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	_, err = nc.Subscribe("tracker.issue.events", func(msg *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := persistEvent(ctx, db, msg.Data); err != nil {
			slog.Error("store activity event", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe events", "error", err)
		os.Exit(1)
	}
	if err = nc.Flush(); err != nil {
		slog.Error("flush event subscription", "error", err)
		os.Exit(1)
	}
	h := &activityHandler{db: db, projectURL: env("PROJECT_SERVICE_URL", "http://project:8082"), client: &http.Client{Timeout: 3 * time.Second}}
	mux := http.NewServeMux()
	mux.Handle("/health", health.Handler(nil))
	mux.Handle("/ready", health.Handler(func() error { return db.Ping() }))
	mux.Handle("/activity", h)
	s := &http.Server{Addr: env("ACTIVITY_ADDR", ":8084"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("activity service listening", "address", s.Addr)
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

func persistEvent(ctx context.Context, db *sql.DB, data []byte) error {
	var e event
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if e.EventID == "" || e.IssueID == 0 || e.ProjectID == 0 || e.ActorID == 0 || (e.EventType != "IssueCreated" && e.EventType != "StatusChanged") {
		return errors.New("invalid issue event")
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "INSERT INTO activity_logs(event_id,project_id,issue_id,actor_id,event_type,payload) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE event_id=VALUES(event_id)", e.EventID, e.ProjectID, e.IssueID, e.ActorID, e.EventType, payload)
	return err
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
