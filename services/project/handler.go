package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type projectHandler struct{ db *sql.DB }
type project struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     uint64 `json:"owner_id"`
}
type projectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 160 {
			http.Error(w, "invalid project", http.StatusBadRequest)
			return
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
		if err = tx.Commit(); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, http.StatusCreated, project{ID: uint64(id), Name: strings.TrimSpace(input.Name), Description: input.Description, OwnerID: userID})
	case http.MethodGet:
		rows, err := h.db.QueryContext(r.Context(), "SELECT p.id, p.name, p.description, p.owner_id FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=? ORDER BY p.id", userID)
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
		err := h.db.QueryRowContext(r.Context(), "SELECT id,name,description,owner_id FROM projects WHERE id=?", id).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID)
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
		_, err := h.db.ExecContext(r.Context(), "UPDATE projects SET name=?,description=? WHERE id=?", strings.TrimSpace(in.Name), in.Description, id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, 200, map[string]bool{"updated": true})
	case http.MethodDelete:
		if !h.isAdmin(r, id, userID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, err := h.db.ExecContext(r.Context(), "DELETE FROM projects WHERE id=?", id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, PATCH, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectHandler) members(w http.ResponseWriter, r *http.Request, projectID, actor uint64) {
	if !h.isAdmin(r, projectID, actor) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			UserID uint64 `json:"user_id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil || body.UserID == 0 {
			http.Error(w, "invalid user_id", 400)
			return
		}
		_, err := h.db.ExecContext(r.Context(), "INSERT INTO project_members (project_id,user_id) VALUES (?,?) ON DUPLICATE KEY UPDATE user_id=VALUES(user_id)", projectID, body.UserID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeProject(w, 201, map[string]bool{"added": true})
	case http.MethodDelete:
		memberID, err := strconv.ParseUint(r.URL.Query().Get("user_id"), 10, 64)
		if err != nil || memberID == 0 {
			http.Error(w, "invalid user_id", 400)
			return
		}
		if memberID == actor {
			http.Error(w, "cannot remove project owner", 400)
			return
		}
		_, err = h.db.ExecContext(r.Context(), "DELETE FROM project_members WHERE project_id=? AND user_id=?", projectID, memberID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		w.WriteHeader(204)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectHandler) checkMember(w http.ResponseWriter, r *http.Request, projectID, userID uint64) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var exists bool
	err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=? AND user_id=?)", projectID, userID).Scan(&exists)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeProject(w, 200, map[string]bool{"member": exists})
}
func (h *projectHandler) isAdmin(r *http.Request, projectID, userID uint64) bool {
	var exists bool
	err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=? AND user_id=? AND role='admin')", projectID, userID).Scan(&exists)
	return err == nil && exists
}
func (h *projectHandler) isMember(r *http.Request, projectID, userID uint64) bool {
	var exists bool
	err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=? AND user_id=?)", projectID, userID).Scan(&exists)
	return err == nil && exists
}
func writeProject(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
