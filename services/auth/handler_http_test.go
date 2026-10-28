package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	query := "SELECT id,password_hash,role,password_reset_required,password_reset_version,active FROM users WHERE email=?"
	mock.ExpectQuery(query).WithArgs("alice@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role", "password_reset_required", "password_reset_version", "active"}).AddRow(17, string(hash), "member", false, 0, true))
	mock.ExpectQuery(query).WithArgs("alice@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role", "password_reset_required", "password_reset_version", "active"}).AddRow(17, string(hash), "member", false, 0, true))
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
	mock.ExpectQuery("SELECT id,email FROM users WHERE active=TRUE AND email LIKE").WithArgs("al%").WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(2, "alice@example.test").AddRow(3, "albert@example.test").AddRow(4, "bob@example.test"))
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

func TestUserDirectoryReturnsPaginatedActiveUsernamesWithoutEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/directory?limit=2", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
	mock.ExpectQuery("SELECT id,email FROM users WHERE active=TRUE AND id>\\? AND id<>\\? ORDER BY id ASC LIMIT \\?").WithArgs(uint64(0), uint64(4), uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(1, "alice@example.test").AddRow(2, "ben@example.test").AddRow(3, "charlie@example.test"))
	request := httptest.NewRequest(http.MethodGet, "/users/directory?limit=2", nil)
	request.Header.Set("X-User-ID", "4")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"username":"alice"`) || !strings.Contains(w.Body.String(), `"username":"ben"`) || !strings.Contains(w.Body.String(), `"next_after_id":2`) || strings.Contains(w.Body.String(), "@example.test") || strings.Contains(w.Body.String(), `"id":4`) {
		t.Fatalf("directory status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserDirectoryRejectsInvalidPagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	for _, path := range []string{"/users/directory?limit=0", "/users/directory?limit=101", "/users/directory?after_id=0"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-User-ID", "4")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, request)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUsernameLookupReturnsNamesForRequestedIDsOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,email,active FROM users WHERE id IN \\(\\?,\\?\\)").WithArgs(uint64(42), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "email", "active"}).AddRow(7, "sam@example.test", true).AddRow(42, "alex@example.test", true))
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	request := httptest.NewRequest(http.MethodPost, "/users/names", strings.NewReader(`{"user_ids":[42,7,42]}`))
	request.Header.Set("X-User-ID", "3")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":42,"username":"alex","active":true`) || !strings.Contains(w.Body.String(), `"id":7,"username":"sam","active":true`) || strings.Contains(w.Body.String(), "@example.test") {
		t.Fatalf("lookup status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Index(w.Body.String(), `"id":42`) > strings.Index(w.Body.String(), `"id":7`) {
		t.Fatalf("lookup did not preserve request order: %s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUsernameLookupRequiresIdentityAndBoundsIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	for _, test := range []struct {
		name string
		body string
		user string
		want int
	}{
		{name: "no identity", body: `{"user_ids":[1]}`, want: http.StatusUnauthorized},
		{name: "zero id", body: `{"user_ids":[0]}`, user: "3", want: http.StatusBadRequest},
		{name: "empty ids", body: `{"user_ids":[]}`, user: "3", want: http.StatusBadRequest},
		{name: "too many IDs", body: `{"user_ids":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77,78,79,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99,100,101]}`, user: "3", want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/users/names", strings.NewReader(test.body))
			if test.user != "" {
				request.Header.Set("X-User-ID", test.user)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, request)
			if w.Code != test.want {
				t.Fatalf("status=%d body=%s, want %d", w.Code, w.Body.String(), test.want)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCanListUsersWithoutPasswordHashes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	request := httptest.NewRequest(http.MethodGet, "/users?limit=20&after_id=5", nil)
	request.Header.Set("X-User-ID", "1")
	request.Header.Set("X-User-Role", "admin")
	mock.ExpectQuery("SELECT id,email,role,created_at,password_reset_required,active FROM users WHERE id>\\?.*LIMIT \\?").WithArgs(uint64(5), uint64(20)).WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "created_at", "password_reset_required", "active"}).AddRow(7, "member@example.test", "member", time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC), false, true))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"email":"member@example.test"`) || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/users", nil)
	request.Header.Set("X-User-ID", "2")
	request.Header.Set("X-User-Role", "member")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin list status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalAdminCanCreateAndDeactivateAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	create := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"email":"new@example.test","temporary_password":"temporary1"}`))
	create.Header.Set("X-User-ID", "1")
	create.Header.Set("X-User-Role", "admin")
	mock.ExpectExec("INSERT INTO users \\(email,password_hash,role,password_reset_required,password_reset_version\\) VALUES \\(\\?,\\?, 'member',TRUE,1\\)").WithArgs("new@example.test", bcryptPassword("temporary1")).WillReturnResult(sqlmock.NewResult(45, 1))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, create)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"must_change_password":true`) || strings.Contains(w.Body.String(), "temporary1") {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	deactivate := httptest.NewRequest(http.MethodPatch, "/users/45/status", strings.NewReader(`{"active":false}`))
	deactivate.Header.Set("X-User-ID", "1")
	deactivate.Header.Set("X-User-Role", "admin")
	mock.ExpectQuery("SELECT active FROM users WHERE id=\\?").WithArgs(uint64(45)).WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	mock.ExpectExec("UPDATE users SET active=\\?,password_reset_version=password_reset_version\\+1 WHERE id=\\?").WithArgs(false, uint64(45)).WillReturnResult(sqlmock.NewResult(0, 1))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, deactivate)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"active":false`) {
		t.Fatalf("deactivate status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalAdminCannotDeactivateSelf(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	request := httptest.NewRequest(http.MethodPatch, "/users/1/status", strings.NewReader(`{"active":false}`))
	request.Header.Set("X-User-ID", "1")
	request.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self-deactivation status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInactiveAccountsCannotLoginOrPassSessionCheck(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("temporary1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,password_hash,role,password_reset_required,password_reset_version,active FROM users WHERE email=\\?").WithArgs("off@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role", "password_reset_required", "password_reset_version", "active"}).AddRow(8, string(hash), "member", false, 2, false))
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"off@example.test","password":"temporary1"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("inactive login status=%d body=%s", w.Code, w.Body.String())
	}
	mock.ExpectQuery("SELECT password_reset_required,password_reset_version,active FROM users WHERE id=\\?").WithArgs(uint64(8)).WillReturnRows(sqlmock.NewRows([]string{"password_reset_required", "password_reset_version", "active"}).AddRow(false, 2, false))
	request := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	request.Header.Set("X-User-ID", "8")
	request.Header.Set("X-Password-Reset-Version", "2")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("inactive session status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInternalAccountStatusEndpointReturnsOnlyActiveFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT active FROM users WHERE id=\\?").WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/users/7/active", nil))
	if w.Code != http.StatusOK || w.Body.String() != "{\"active\":true}\n" {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStatusReadsCurrentPasswordResetFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	mock.ExpectQuery("SELECT password_reset_required,password_reset_version,active FROM users WHERE id=\\?").WithArgs(uint64(91)).WillReturnRows(sqlmock.NewRows([]string{"password_reset_required", "password_reset_version", "active"}).AddRow(true, 4, true))
	request := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	request.Header.Set("X-User-ID", "91")
	request.Header.Set("X-Password-Reset-Version", "4")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"password_reset_required":true`) || !strings.Contains(w.Body.String(), `"password_reset_version":4`) {
		t.Fatalf("session status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminPasswordResetHashesTemporaryPasswordAndRequiresRotation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	request := httptest.NewRequest(http.MethodPost, "/users/8/password-reset", strings.NewReader(`{"temporary_password":"temporary1"}`))
	request.Header.Set("X-User-ID", "1")
	request.Header.Set("X-User-Role", "admin")
	mock.ExpectExec("UPDATE users SET password_hash=\\?,password_reset_required=TRUE,password_reset_version=password_reset_version\\+1 WHERE id=\\?").WithArgs(bcryptPassword("temporary1"), uint64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"must_change_password":true`) || strings.Contains(w.Body.String(), "temporary1") {
		t.Fatalf("reset status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMustChangePasswordFlowUpdatesHashAndIssuesNormalToken(t *testing.T) {
	secret := []byte("test-secret-test-secret-test-secret")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := NewHandler(db, secret)
	request := httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader(`{"new_password":"newpassword1"}`))
	request.Header.Set("X-User-ID", "17")
	request.Header.Set("X-Must-Change-Password", "true")
	request.Header.Set("X-Password-Reset-Version", "3")
	temporaryHash, err := bcrypt.GenerateFromPassword([]byte("temporary1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\? AND password_reset_required=TRUE AND password_reset_version=\\?").WithArgs(uint64(17), uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(temporaryHash)))
	mock.ExpectExec("UPDATE users SET password_hash=\\?,password_reset_required=FALSE WHERE id=\\? AND password_reset_required=TRUE AND password_reset_version=\\?").WithArgs(bcryptPassword("newpassword1"), uint64(17), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT role,password_reset_version FROM users WHERE id=\\?").WithArgs(uint64(17)).WillReturnRows(sqlmock.NewRows([]string{"role", "password_reset_version"}).AddRow("member", 3))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"must_change_password":false`) {
		t.Fatalf("change password status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id, role, mustChange, err := sharedauth.VerifyTokenWithPasswordReset(response.Token, secret)
	if err != nil || id != 17 || role != "member" || mustChange {
		t.Fatalf("new token claims=(%d,%q,%t), err=%v", id, role, mustChange, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestForcedPasswordChangeRejectsTemporaryPasswordReuse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	temporaryHash, err := bcrypt.GenerateFromPassword([]byte("temporary1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT password_hash FROM users WHERE id=\\? AND password_reset_required=TRUE AND password_reset_version=\\?").WithArgs(uint64(17), uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(temporaryHash)))
	h := NewHandler(db, []byte("test-secret-test-secret-test-secret"))
	request := httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader(`{"new_password":"temporary1"}`))
	request.Header.Set("X-User-ID", "17")
	request.Header.Set("X-Must-Change-Password", "true")
	request.Header.Set("X-Password-Reset-Version", "3")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "differ from the temporary password") {
		t.Fatalf("reuse status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginMarksPasswordResetSessionForForcedChange(t *testing.T) {
	secret := []byte("test-secret-test-secret-test-secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("temporary1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,password_hash,role,password_reset_required,password_reset_version,active FROM users WHERE email=\\?").WithArgs("member@example.test").WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role", "password_reset_required", "password_reset_version", "active"}).AddRow(33, string(hash), "member", true, 2, true))
	h := NewHandler(db, secret)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"member@example.test","password":"temporary1"}`)))
	var response struct {
		AccessToken        string `json:"access_token"`
		MustChangePassword bool   `json:"must_change_password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id, role, mustChange, tokenErr := sharedauth.VerifyTokenWithPasswordReset(response.AccessToken, secret)
	if w.Code != http.StatusOK || !response.MustChangePassword || tokenErr != nil || id != 33 || role != "member" || !mustChange {
		t.Fatalf("status=%d response=%#v claims=(%d,%q,%t) tokenErr=%v", w.Code, response, id, role, mustChange, tokenErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
