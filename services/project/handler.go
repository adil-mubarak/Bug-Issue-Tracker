package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type projectHandler struct {
	db      *sql.DB
	authURL string
	client  *http.Client
}
type project struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     uint64 `json:"owner_id"`
}
type projectInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MemberIDs   []uint64 `json:"member_ids"`
}
type projectMember struct {
	UserID    uint64 `json:"user_id"`
	Role      string `json:"role"`
	Synthetic bool   `json:"synthetic,omitempty"`
}

func (h *projectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseUint(r.Header.Get("X-User-ID"), 10, 64)
	if err != nil || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/projects"), "/")
	if path == "" {
		h.collection(w, r, userID)
		return
	}
	parts := strings.Split(path, "/")
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 3 && parts[1] == "members" && parts[2] == "check" {
		h.checkMember(w, r, id, userID)
		return
	}
	if len(parts) == 2 && parts[1] == "members" {
		h.members(w, r, id, userID)
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	h.item(w, r, id, userID)
}

func (h *projectHandler) collection(w http.ResponseWriter, r *http.Request, userID uint64) {
	switch r.Method {
	case http.MethodPost:
		var input projectInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 160 || len(input.MemberIDs) > 50 {
			http.Error(w, "invalid project", http.StatusBadRequest)
			return
		}
		memberIDs := make([]uint64, 0, len(input.MemberIDs))
		seenMembers := make(map[uint64]struct{}, len(input.MemberIDs))
		for _, memberID := range input.MemberIDs {
			if memberID == 0 {
				http.Error(w, "invalid member_ids", http.StatusBadRequest)
				return
			}
			if _, exists := seenMembers[memberID]; exists {
				http.Error(w, "duplicate member_id", http.StatusBadRequest)
				return
			}
			seenMembers[memberID] = struct{}{}
			if memberID == userID {
				continue
			}
			memberIDs = append(memberIDs, memberID)
		}
		for _, memberID := range memberIDs {
			active, err := h.accountIsActive(r.Context(), memberID)
			if err != nil {
				http.Error(w, "account service unavailable", http.StatusBadGateway)
				return
			}
			if !active {
				http.Error(w, "member account is inactive or does not exist", http.StatusBadRequest)
				return
			}
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(r.Context(), "INSERT INTO projects (name, description, owner_id) VALUES (?, ?, ?)", strings.TrimSpace(input.Name), input.Description, userID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		id, _ := result.LastInsertId()
		if _, err = tx.ExecContext(r.Context(), "INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, 'admin')", id, userID); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		for _, memberID := range memberIDs {
			if _, err = tx.ExecContext(r.Context(), "INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, 'member')", id, memberID); err != nil {
				http.Error(w, "database error", 500)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, http.StatusCreated, project{ID: uint64(id), Name: strings.TrimSpace(input.Name), Description: input.Description, OwnerID: userID})
	case http.MethodGet:
		query := "SELECT p.id, p.name, p.description, p.owner_id FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=? AND p.deleted_at IS NULL ORDER BY p.id"
		args := []any{userID}
		if isPlatformAdmin(r) {
			query = "SELECT id,name,description,owner_id FROM projects WHERE deleted_at IS NULL ORDER BY id"
			args = nil
		} else if r.URL.Query().Get("scope") == "all" {
			http.Error(w, "global admin required", http.StatusForbidden)
			return
		}
		rows, err := h.db.QueryContext(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		items := []project{}
		for rows.Next() {
			var p project
			if rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID) != nil {
				http.Error(w, "database error", 500)
				return
			}
			items = append(items, p)
		}
		if rows.Err() != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, 200, items)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectHandler) item(w http.ResponseWriter, r *http.Request, id, userID uint64) {
	switch r.Method {
	case http.MethodGet:
		if !h.isMember(r, id, userID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var p project
		err := h.db.QueryRowContext(r.Context(), "SELECT id,name,description,owner_id FROM projects WHERE id=? AND deleted_at IS NULL", id).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, 200, p)
	case http.MethodPut, http.MethodPatch:
		if !h.isAdmin(r, id, userID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var in projectInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
			http.Error(w, "invalid project", 400)
			return
		}
		_, err := h.db.ExecContext(r.Context(), "UPDATE projects SET name=?,description=? WHERE id=? AND deleted_at IS NULL", strings.TrimSpace(in.Name), in.Description, id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, 200, map[string]bool{"updated": true})
	case http.MethodDelete:
		var ownerID uint64
		if err := h.db.QueryRowContext(r.Context(), "SELECT owner_id FROM projects WHERE id=? AND deleted_at IS NULL", id).Scan(&ownerID); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if !isPlatformAdmin(r) && ownerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		result, err := h.db.ExecContext(r.Context(), "UPDATE projects SET deleted_at=CURRENT_TIMESTAMP WHERE id=? AND deleted_at IS NULL", id)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if affected, err := result.RowsAffected(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		} else if affected == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, PATCH, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectHandler) members(w http.ResponseWriter, r *http.Request, projectID, actor uint64) {
	switch r.Method {
	case http.MethodGet:
		if isPlatformAdmin(r) {
			var projectExists bool
			if err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL)", projectID).Scan(&projectExists); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if !projectExists {
				http.NotFound(w, r)
				return
			}
		}
		if !isPlatformAdmin(r) && !h.isMember(r, projectID, actor) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rows, err := h.db.QueryContext(r.Context(), "SELECT user_id,role FROM project_members WHERE project_id=? ORDER BY user_id", projectID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		members := []projectMember{}
		adminListed := false
		for rows.Next() {
			var member projectMember
			if err := rows.Scan(&member.UserID, &member.Role); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			adminListed = adminListed || member.UserID == actor
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if isPlatformAdmin(r) && !adminListed {
			members = append(members, projectMember{UserID: actor, Role: "member", Synthetic: true})
		}
		writeProject(w, http.StatusOK, members)
	case http.MethodPost:
		if !h.canManageProject(r, projectID, actor) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			UserID uint64 `json:"user_id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil || body.UserID == 0 {
			http.Error(w, "invalid user_id", 400)
			return
		}
		active, err := h.accountIsActive(r.Context(), body.UserID)
		if err != nil {
			http.Error(w, "account service unavailable", http.StatusBadGateway)
			return
		}
		if !active {
			http.Error(w, "user account is inactive or does not exist", http.StatusBadRequest)
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		var ownerID uint64
		if err := tx.QueryRowContext(r.Context(), "SELECT owner_id FROM projects WHERE id=? AND deleted_at IS NULL FOR UPDATE", projectID).Scan(&ownerID); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if !isPlatformAdmin(r) {
			var actorRole string
			if err := tx.QueryRowContext(r.Context(), "SELECT role FROM project_members WHERE project_id=? AND user_id=? FOR UPDATE", projectID, actor).Scan(&actorRole); err == sql.ErrNoRows {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			} else if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if actorRole != "admin" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(), "INSERT INTO project_members (project_id,user_id) VALUES (?,?) ON DUPLICATE KEY UPDATE user_id=VALUES(user_id)", projectID, body.UserID); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		writeProject(w, 201, map[string]bool{"added": true})
	case http.MethodPatch:
		if !h.canManageProject(r, projectID, actor) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			UserID uint64 `json:"user_id"`
			Role   string `json:"role"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil || body.UserID == 0 || (body.Role != "admin" && body.Role != "member") {
			http.Error(w, "invalid project member", http.StatusBadRequest)
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		var ownerID uint64
		if err := tx.QueryRowContext(r.Context(), "SELECT owner_id FROM projects WHERE id=? AND deleted_at IS NULL FOR UPDATE", projectID).Scan(&ownerID); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if !isPlatformAdmin(r) {
			var actorRole string
			if err := tx.QueryRowContext(r.Context(), "SELECT role FROM project_members WHERE project_id=? AND user_id=? FOR UPDATE", projectID, actor).Scan(&actorRole); err == sql.ErrNoRows {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			} else if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if actorRole != "admin" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		if ownerID == body.UserID && body.Role != "admin" {
			http.Error(w, "project owner must remain an admin", http.StatusBadRequest)
			return
		}
		var currentRole string
		if err := tx.QueryRowContext(r.Context(), "SELECT role FROM project_members WHERE project_id=? AND user_id=? FOR UPDATE", projectID, body.UserID).Scan(&currentRole); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if currentRole == "admin" && body.Role == "member" {
			var adminCount int
			if err := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_members WHERE project_id=? AND role='admin'", projectID).Scan(&adminCount); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if adminCount <= 1 {
				http.Error(w, "project must keep at least one admin", http.StatusConflict)
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(), "UPDATE project_members SET role=? WHERE project_id=? AND user_id=?", body.Role, projectID, body.UserID); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		writeProject(w, http.StatusOK, map[string]bool{"updated": true})
	case http.MethodDelete:
		if !h.canManageProject(r, projectID, actor) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		memberID, err := strconv.ParseUint(r.URL.Query().Get("user_id"), 10, 64)
		if err != nil || memberID == 0 {
			http.Error(w, "invalid user_id", 400)
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		var ownerID uint64
		if err := tx.QueryRowContext(r.Context(), "SELECT owner_id FROM projects WHERE id=? AND deleted_at IS NULL FOR UPDATE", projectID).Scan(&ownerID); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if !isPlatformAdmin(r) {
			var actorRole string
			if err := tx.QueryRowContext(r.Context(), "SELECT role FROM project_members WHERE project_id=? AND user_id=? FOR UPDATE", projectID, actor).Scan(&actorRole); err == sql.ErrNoRows {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			} else if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if actorRole != "admin" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		if memberID == ownerID {
			http.Error(w, "cannot remove project owner", 400)
			return
		}
		var role string
		if err := tx.QueryRowContext(r.Context(), "SELECT role FROM project_members WHERE project_id=? AND user_id=? FOR UPDATE", projectID, memberID).Scan(&role); err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if role == "admin" {
			var adminCount int
			if err := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_members WHERE project_id=? AND role='admin'", projectID).Scan(&adminCount); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if adminCount <= 1 {
				http.Error(w, "project must keep at least one admin", http.StatusConflict)
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM project_members WHERE project_id=? AND user_id=?", projectID, memberID); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(204)
	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectHandler) checkMember(w http.ResponseWriter, r *http.Request, projectID, userID uint64) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	targetID := userID
	if rawTarget := r.URL.Query().Get("user_id"); rawTarget != "" {
		parsed, err := strconv.ParseUint(rawTarget, 10, 64)
		if err != nil || parsed == 0 {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}
		targetID = parsed
		if targetID != userID && !h.isMember(r, projectID, userID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	if targetID == userID && isPlatformAdmin(r) {
		var projectExists bool
		if err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL)", projectID).Scan(&projectExists); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		writeProject(w, http.StatusOK, map[string]bool{"member": projectExists})
		return
	}
	var exists bool
	err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM project_members m JOIN projects p ON p.id=m.project_id WHERE m.project_id=? AND m.user_id=? AND p.deleted_at IS NULL)", projectID, targetID).Scan(&exists)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if exists && targetID != userID {
		active, err := h.accountIsActive(r.Context(), targetID)
		if err != nil {
			http.Error(w, "account service unavailable", http.StatusBadGateway)
			return
		}
		exists = active
	}
	writeProject(w, 200, map[string]bool{"member": exists})
}
func (h *projectHandler) isAdmin(r *http.Request, projectID, userID uint64) bool {
	var exists bool
	err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM project_members m JOIN projects p ON p.id=m.project_id WHERE m.project_id=? AND m.user_id=? AND m.role='admin' AND p.deleted_at IS NULL)", projectID, userID).Scan(&exists)
	return err == nil && exists
}
func (h *projectHandler) canManageProject(r *http.Request, projectID, userID uint64) bool {
	return isPlatformAdmin(r) || h.isAdmin(r, projectID, userID)
}
func isPlatformAdmin(r *http.Request) bool { return r.Header.Get("X-User-Role") == "admin" }

func (h *projectHandler) accountIsActive(parent context.Context, userID uint64) (bool, error) {
	if h.authURL == "" {
		return false, fmt.Errorf("auth service URL is not configured")
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	endpoint := fmt.Sprintf("%s/internal/users/%d/active", strings.TrimRight(h.authURL, "/"), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	client := h.client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("auth service returned %s", resp.Status)
	}
	var status struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return false, err
	}
	return status.Active, nil
}

func (h *projectHandler) isMember(r *http.Request, projectID, userID uint64) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM project_members m JOIN projects p ON p.id=m.project_id WHERE m.project_id=? AND m.user_id=? AND p.deleted_at IS NULL)"
	if isPlatformAdmin(r) {
		query = "SELECT EXISTS(SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL)"
		err := h.db.QueryRowContext(r.Context(), query, projectID).Scan(&exists)
		return err == nil && exists
	}
	err := h.db.QueryRowContext(r.Context(), query, projectID, userID).Scan(&exists)
	return err == nil && exists
}
func writeProject(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
