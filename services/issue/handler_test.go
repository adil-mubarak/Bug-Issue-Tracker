package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func membershipServer(member bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/4/members/check" && r.URL.Path != "/projects/9/members/check" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if member {
			_, _ = w.Write([]byte(`{"member":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"member":false}`))
	}))
}

func newIssueTestHandler(t *testing.T, db *sql.DB, member bool) (*issueHandler, func()) {
	t.Helper()
	server := membershipServer(member)
	return &issueHandler{db: db, projectURL: server.URL, client: server.Client()}, server.Close
}

func sendIssueRequest(h *issueHandler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-User-ID", "12")
	r.Header.Set("X-Request-ID", "test-request-id")
	h.ServeHTTP(w, r)
	return w
}

func TestIssueCreateAndMembershipAuthorization(t *testing.T) {
	t.Run("member can create", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		h, closeServer := newIssueTestHandler(t, db, true)
		defer closeServer()
		mock.ExpectExec("INSERT INTO issues").WithArgs(uint64(4), "Title", "Description", "medium", "todo", nil, uint64(12)).WillReturnResult(sqlmock.NewResult(30, 1))
		w := sendIssueRequest(h, http.MethodPost, "/issues", `{"project_id":4,"title":"Title","description":"Description"}`)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":30`) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("nonmember is forbidden", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		h, closeServer := newIssueTestHandler(t, db, false)
		defer closeServer()
		w := sendIssueRequest(h, http.MethodPost, "/issues", `{"project_id":4,"title":"Title"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid input is rejected", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		h, closeServer := newIssueTestHandler(t, db, true)
		defer closeServer()
		for _, body := range []string{`{}`, `{"project_id":4,"title":" "}`, `{"project_id":4,"title":"Title","priority":"urgent"}`} {
			w := sendIssueRequest(h, http.MethodPost, "/issues", body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("body=%s status=%d", body, w.Code)
			}
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestIssueListValidatesCursorAndLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, closeServer := newIssueTestHandler(t, db, true)
	defer closeServer()
	for _, path := range []string{"/issues?project_id=4&after_id=abc", "/issues?project_id=4&limit=0", "/issues?project_id=4&limit=abc"} {
		w := sendIssueRequest(h, http.MethodGet, path, "")
		if w.Code != http.StatusBadRequest {
			t.Errorf("path=%s status=%d", path, w.Code)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueGetUpdateAndDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, closeServer := newIssueTestHandler(t, db, true)
	defer closeServer()

	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectQuery("SELECT id,project_id,title,description,priority,status,assignee_id,reporter_id FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "title", "description", "priority", "status", "assignee_id", "reporter_id"}).AddRow(30, 4, "A bug", "Details", "high", "todo", nil, 12))
	w := sendIssueRequest(h, http.MethodGet, "/issues/30", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"A bug"`) {
		t.Fatalf("get status=%d body=%s", w.Code, w.Body.String())
	}

	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectQuery("SELECT status FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("todo"))
	mock.ExpectExec("UPDATE issues SET title=").WithArgs("Fixed", "Details", "high", "in_progress", nil, uint64(30)).WillReturnResult(sqlmock.NewResult(0, 1))
	w = sendIssueRequest(h, http.MethodPatch, "/issues/30", `{"title":"Fixed","description":"Details","priority":"high","status":"in_progress"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", w.Code, w.Body.String())
	}

	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectExec("DELETE FROM issues WHERE id=").WithArgs(uint64(30)).WillReturnResult(sqlmock.NewResult(0, 1))
	w = sendIssueRequest(h, http.MethodDelete, "/issues/30", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueDependencyCycleRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, closeServer := newIssueTestHandler(t, db, true)
	defer closeServer()
	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectQuery("SELECT project_id FROM issues WHERE id=").WithArgs(uint64(2)).WillReturnRows(sqlmock.NewRows([]string{"project_id"}).AddRow(4))
	mock.ExpectQuery("SELECT i.id,d.depends_on_issue_id FROM issues").WithArgs(uint64(4)).WillReturnRows(sqlmock.NewRows([]string{"id", "depends_on_issue_id"}).AddRow(2, 1))
	w := sendIssueRequest(h, http.MethodPost, "/issues/1/dependencies", `{"depends_on_issue_id":2}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueWorkOrderEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, closeServer := newIssueTestHandler(t, db, true)
	defer closeServer()
	mock.ExpectQuery("SELECT i.id,d.depends_on_issue_id FROM issues i LEFT JOIN issue_dependencies").WithArgs(uint64(4)).WillReturnRows(sqlmock.NewRows([]string{"id", "depends_on_issue_id"}).AddRow(1, nil).AddRow(2, 1).AddRow(3, 2))
	w := sendIssueRequest(h, http.MethodGet, "/issues/work-order?project_id=4", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"work_order":[1,2,3]`) {
		t.Fatalf("work order status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLabelAutocompleteIsMembershipScoped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, closeServer := newIssueTestHandler(t, db, true)
	defer closeServer()
	mock.ExpectQuery("SELECT name FROM labels WHERE project_id=").WithArgs(uint64(4), "ba%").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Backend").AddRow("basecamp").AddRow("frontend"))
	w := sendIssueRequest(h, http.MethodGet, "/issues/labels/autocomplete?project_id=4&q=ba", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Backend") || !strings.Contains(w.Body.String(), "basecamp") || strings.Contains(w.Body.String(), "frontend") {
		t.Fatalf("autocomplete status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueRoutesRequireGatewayIdentity(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &issueHandler{db: db}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/issues?project_id=4", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}
