package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"

	sharedauth "github.com/adil/bug-issue-tracker/pkg/auth"
)

type bcryptPassword string

func (p bcryptPassword) Match(value driver.Value) bool {
	hash, ok := value.(string)
	return ok && bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

func TestRegisterHashesPasswordAndNormalizesEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO users").WithArgs("alice@example.test", bcryptPassword("securepass1"), "member").WillReturnResult(sqlmock.NewResult(7, 1))
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":" Alice@Example.Test ","password":"securepass1"}`))
	h.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["email"] != "alice@example.test" || response["id"] != float64(7) {
		t.Fatalf("unexpected response: %#v", response)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type=%q", w.Header().Get("Content-Type"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsInvalidInputAndDuplicateEmail(t *testing.T) {
	t.Run("invalid fields rejected before database", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
		for _, body := range []string{`{"email":"not-an-email","password":"securepass1"}`, `{"email":"a@example.test","password":"short"}`, `{"email":"a@example.test","password":"12345678","admin":true}`} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body)))
			if w.Code != http.StatusBadRequest {
				t.Errorf("body %s: status=%d", body, w.Code)
			}
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("duplicate email is conflict", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectExec("INSERT INTO users").WithArgs("alice@example.test", sqlmock.AnyArg(), "member").WillReturnError(errors.New("Error 1062: Duplicate entry"))
		h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"alice@example.test","password":"securepass1"}`)))
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoginValidatesPasswordAndIssuesJWT(t *testing.T) {
	secret := []byte("test-secret-test-secret-test-secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("securepass1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT id, password_hash, role FROM users WHERE email = ?"
	mock.ExpectQuery(query).WithArgs("alice@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role"}).AddRow(17, string(hash), "member"))
	mock.ExpectQuery(query).WithArgs("alice@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role"}).AddRow(17, string(hash), "member"))
	h := NewHandler(db, secret)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":" Alice@Example.Test ","password":"securepass1"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id, role, err := sharedauth.VerifyToken(response.AccessToken, secret)
	if err != nil || id != 17 || role != "member" {
		t.Fatalf("claims=(%d,%q), err=%v", id, role, err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"alice@example.test","password":"wrongpass"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAuthMethodAndMalformedJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{} {}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d", w.Code)
	}
}

func TestUsernameAutocompleteRequiresIdentityAndUsesTrie(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/autocomplete?q=al", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
	mock.ExpectQuery("SELECT id,email FROM users WHERE email LIKE").WithArgs("al%").WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(2, "alice@example.test").AddRow(3, "albert@example.test").AddRow(4, "bob@example.test"))
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/autocomplete?q=al", nil)
	r.Header.Set("X-User-ID", "4")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"username":"alice"`) || !strings.Contains(w.Body.String(), `"username":"albert"`) || strings.Contains(w.Body.String(), "bob") || strings.Contains(w.Body.String(), "@example") {
		t.Fatalf("autocomplete status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
