package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPersistEventAndRejectInvalidPayloads(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	data := []byte(`{"event_id":"evt-1","issue_id":8,"project_id":4,"actor_id":12,"event_type":"IssueCreated","status":"todo"}`)
	mock.ExpectExec("INSERT INTO activity_logs").WithArgs("evt-1", uint64(4), uint64(8), uint64(12), "IssueCreated", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := persistEvent(context.Background(), db, data); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO activity_logs").WithArgs("evt-1", uint64(4), uint64(8), uint64(12), "IssueCreated", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := persistEvent(context.Background(), db, data); err != nil {
		t.Fatalf("duplicate delivery should be idempotent: %v", err)
	}
	for _, invalid := range [][]byte{[]byte("not json"), []byte(`{"event_id":"","issue_id":8,"project_id":4,"actor_id":12,"event_type":"IssueCreated"}`), []byte(`{"event_id":"evt-2","issue_id":8,"project_id":4,"actor_id":12,"event_type":"Unknown"}`)} {
		if err := persistEvent(context.Background(), db, invalid); err == nil {
			t.Errorf("expected invalid event error for %s", invalid)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivityFeedRequiresProjectMembership(t *testing.T) {
	for _, member := range []bool{false, true} {
		t.Run(map[bool]string{false: "forbidden member", true: "member feed"}[member], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/projects/4/members/check" || r.Header.Get("X-User-ID") != "12" || r.Header.Get("X-Request-ID") != "activity-test-request" {
					http.Error(w, "bad internal request", 400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if member {
					_, _ = w.Write([]byte(`{"member":true}`))
				} else {
					_, _ = w.Write([]byte(`{"member":false}`))
				}
			}))
			defer project.Close()
			u, err := url.Parse(project.URL)
			if err != nil {
				t.Fatal(err)
			}
			h := &activityHandler{db: db, projectURL: u.String(), client: project.Client()}
			if member {
				mock.ExpectQuery("SELECT id,project_id,issue_id,actor_id,event_type,payload,occurred_at FROM activity_logs").WithArgs(uint64(4), 50).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "issue_id", "actor_id", "event_type", "payload", "occurred_at"}).AddRow(2, 4, 8, 12, "IssueCreated", []byte(`{"event_id":"evt-1"}`), []byte("2026-01-01 00:00:00")))
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/activity?project_id=4", nil)
			r.Header.Set("X-User-ID", "12")
			r.Header.Set("X-Request-ID", "activity-test-request")
			h.ServeHTTP(w, r)
			if member {
				if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"issue_id":8`) {
					t.Fatalf("feed status=%d body=%s", w.Code, w.Body.String())
				}
			} else if w.Code != http.StatusForbidden {
				t.Fatalf("nonmember status=%d", w.Code)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActivityFeedInvalidIdentityAndLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	project := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"member":true}`))
	}))
	defer project.Close()
	h := &activityHandler{db: db, projectURL: project.URL, client: project.Client()}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/activity?project_id=4", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing identity status=%d", w.Code)
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/activity?project_id=4&limit=500", nil)
	r.Header.Set("X-User-ID", "12")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status=%d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
