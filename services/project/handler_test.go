package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func projectRequest(h *projectHandler, method, path, userID, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		r.Header.Set("X-User-ID", userID)
	}
	h.ServeHTTP(w, r)
	return w
}

func TestProjectCreateTransactionAndAuthentication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &projectHandler{db: db}
	if got := projectRequest(h, http.MethodGet, "/projects", "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("missing user status=%d", got)
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO projects").WithArgs("Roadmap", "Quarterly plan", uint64(22)).WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec("INSERT INTO project_members").WithArgs(int64(8), uint64(22)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	w := projectRequest(h, http.MethodPost, "/projects", "22", `{"name":" Roadmap ","description":"Quarterly plan"}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"owner_id":22`) {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectReadAndMembershipChecks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &projectHandler{db: db}
	mock.ExpectQuery("SELECT p.id, p.name, p.description, p.owner_id").WithArgs(uint64(4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "owner_id"}).AddRow(3, "App", "Description", 4))
	w := projectRequest(h, http.MethodGet, "/projects", "4", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"App"`) {
		t.Fatalf("list response %d %s", w.Code, w.Body.String())
	}
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM project_members").WithArgs(uint64(3), uint64(4)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	w = projectRequest(h, http.MethodGet, "/projects/3/members/check", "4", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"member":true`) {
		t.Fatalf("membership response %d %s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectUpdateAndMemberAdministration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &projectHandler{db: db}
	adminQuery := "SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=? AND user_id=? AND role='admin')"
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	if got := projectRequest(h, http.MethodPut, "/projects/9", "5", `{"name":"Nope"}`).Code; got != http.StatusForbidden {
		t.Fatalf("non-admin update status=%d", got)
	}
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("UPDATE projects SET name=").WithArgs("Updated", "Details", uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	if got := projectRequest(h, http.MethodPut, "/projects/9", "5", `{"name":"Updated","description":"Details"}`).Code; got != 200 {
		t.Fatalf("update status=%d", got)
	}
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("INSERT INTO project_members").WithArgs(uint64(9), uint64(6)).WillReturnResult(sqlmock.NewResult(0, 1))
	if got := projectRequest(h, http.MethodPost, "/projects/9/members", "5", `{"user_id":6}`).Code; got != http.StatusCreated {
		t.Fatalf("add member status=%d", got)
	}
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DELETE FROM project_members").WithArgs(uint64(9), uint64(6)).WillReturnResult(sqlmock.NewResult(0, 1))
	if got := projectRequest(h, http.MethodDelete, "/projects/9/members?user_id=6", "5", "").Code; got != http.StatusNoContent {
		t.Fatalf("remove member status=%d", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectMemberCannotReadPrivateProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &projectHandler{db: db}
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM project_members").WithArgs(uint64(2), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	if got := projectRequest(h, http.MethodGet, "/projects/2", "7", "").Code; got != http.StatusForbidden {
		t.Fatalf("non-member read status=%d", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDeleteRequiresAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &projectHandler{db: db}
	adminQuery := "SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=? AND user_id=? AND role='admin')"
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(6)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	if got := projectRequest(h, http.MethodDelete, "/projects/9", "6", "").Code; got != http.StatusForbidden {
		t.Fatalf("non-admin delete status=%d", got)
	}
	mock.ExpectQuery(regexp.QuoteMeta(adminQuery)).WithArgs(uint64(9), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DELETE FROM projects WHERE id=").WithArgs(uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	if got := projectRequest(h, http.MethodDelete, "/projects/9", "5", "").Code; got != http.StatusNoContent {
		t.Fatalf("admin delete status=%d", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
