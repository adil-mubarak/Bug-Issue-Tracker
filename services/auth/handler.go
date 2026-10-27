package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	sharedauth "github.com/adil/bug-issue-tracker/pkg/auth"
	"github.com/adil/bug-issue-tracker/pkg/trie"
)

type Handler struct {
	db     *sql.DB
	secret []byte
}

func NewHandler(db *sql.DB, secret []byte) *Handler { return &Handler{db: db, secret: secret} }

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	allowedMethod := r.Method == http.MethodPost
	if r.URL.Path == "/users/autocomplete" {
		allowedMethod = r.Method == http.MethodGet
	}
	if !allowedMethod {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/auth/register":
		h.register(w, r)
	case "/auth/login":
		h.login(w, r)
	case "/users/autocomplete":
		h.autocompleteUsers(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) autocompleteUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, err := strconv.ParseUint(r.Header.Get("X-User-ID"), 10, 64)
	if err != nil || uid == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	prefix := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	rows, err := h.db.QueryContext(r.Context(), "SELECT id,email FROM users WHERE email LIKE ? ORDER BY email LIMIT 100", strings.ReplaceAll(prefix, "%", "\\%")+"%")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search users")
		return
	}
	defer rows.Close()
	index := trie.New()
	userIDs := make(map[string]uint64)
	for rows.Next() {
		var email string
		var userID uint64
		if err := rows.Scan(&userID, &email); err != nil {
			writeError(w, http.StatusInternalServerError, "could not search users")
			return
		}
		username := strings.SplitN(email, "@", 2)[0]
		index.Insert(username)
		userIDs[strings.ToLower(username)] = userID
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not search users")
		return
	}
	results := make([]map[string]any, 0, 10)
	for _, username := range index.Search(prefix, 10) {
		results = append(results, map[string]any{"id": userIDs[strings.ToLower(username)], "username": username})
	}
	writeJSON(w, results)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeCredentials(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !strings.Contains(input.Email, "@") || len(input.Email) > 320 || len(input.Password) < 8 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "email or password does not meet requirements")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register user")
		return
	}
	role := "member"
	var count int
	if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err == nil && count == 0 {
		role = "admin"
	}
	result, err := h.db.ExecContext(r.Context(), "INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)", input.Email, string(hash), role)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not register user")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register user")
		return
	}
	writeJSONStatus(w, http.StatusCreated, map[string]any{"id": id, "email": input.Email, "role": role})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeCredentials(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	var userID uint64
	var role string
	var hash string
	err := h.db.QueryRowContext(r.Context(), "SELECT id, password_hash, role FROM users WHERE email = ?", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&userID, &hash, &role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, err := sharedauth.IssueToken(userID, role, h.secret, time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	writeJSON(w, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600})
}

func decodeCredentials(w http.ResponseWriter, r *http.Request, dst *credentials) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSONStatus(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
