package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/adil/bug-issue-tracker/pkg/graph"
	"github.com/adil/bug-issue-tracker/pkg/lru"
	"github.com/adil/bug-issue-tracker/pkg/trie"
	"github.com/nats-io/nats.go"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type issueHandler struct {
	db          *sql.DB
	projectURL  string
	client      *http.Client
	events      *nats.Conn
	memberships *lru.Cache[string, membershipEntry]
}
type membershipEntry struct {
	allowed bool
	expires time.Time
}
type issue struct {
	ID          uint64  `json:"id"`
	ProjectID   uint64  `json:"project_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	Status      string  `json:"status"`
	AssigneeID  *uint64 `json:"assignee_id"`
	ReporterID  uint64  `json:"reporter_id"`
}
type issueInput struct {
	ProjectID   uint64  `json:"project_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	Status      string  `json:"status"`
	AssigneeID  *uint64 `json:"assignee_id"`
}

func (h *issueHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.ParseUint(r.Header.Get("X-User-ID"), 10, 64)
	if err != nil || uid == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/issues"), "/")
	if path == "labels/autocomplete" {
		h.labelAutocomplete(w, r, uid)
		return
	}
	if path == "work-order" {
		h.workOrder(w, r, uid)
		return
	}
	if path == "" {
		switch r.Method {
		case http.MethodPost:
			h.create(w, r, uid)
		case http.MethodGet:
			h.list(w, r, uid)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	id, err := strconv.ParseUint(strings.Split(path, "/")[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var projectID uint64
	if err := h.db.QueryRowContext(r.Context(), "SELECT project_id FROM issues WHERE id=?", id).Scan(&projectID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if !h.isMember(r, projectID, uid) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if strings.Contains(path, "/") {
		if strings.HasSuffix(path, "/dependencies") {
			h.dependencies(w, r, id, projectID)
			return
		}
		h.labels(w, r, id)
		return
	}
	h.item(w, r, id)
}

func (h *issueHandler) labelAutocomplete(w http.ResponseWriter, r *http.Request, uid uint64) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID, err := strconv.ParseUint(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}
	if !h.isMember(r, projectID, uid) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	prefix := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := h.db.QueryContext(r.Context(), "SELECT name FROM labels WHERE project_id=? AND name LIKE ? ORDER BY name LIMIT 100", projectID, strings.ReplaceAll(strings.ReplaceAll(prefix, "\\", "\\\\"), "%", "\\%")+"%")
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	index := trie.New()
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		index.Insert(label)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	writeIssue(w, http.StatusOK, index.Search(prefix, 10))
}

func (h *issueHandler) dependencies(w http.ResponseWriter, r *http.Request, issueID, projectID uint64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := h.db.QueryContext(r.Context(), "SELECT depends_on_issue_id FROM issue_dependencies WHERE issue_id=? ORDER BY depends_on_issue_id", issueID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		ids := []uint64{}
		for rows.Next() {
			var id uint64
			if rows.Scan(&id) != nil {
				http.Error(w, "database error", 500)
				return
			}
			ids = append(ids, id)
		}
		writeIssue(w, 200, ids)
	case http.MethodPost:
		var input struct {
			DependsOn uint64 `json:"depends_on_issue_id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.DependsOn == 0 || input.DependsOn == issueID {
			http.Error(w, "invalid dependency", 400)
			return
		}
		var otherProject uint64
		if err := h.db.QueryRowContext(r.Context(), "SELECT project_id FROM issues WHERE id=?", input.DependsOn).Scan(&otherProject); err != nil || otherProject != projectID {
			http.Error(w, "dependency issue must exist in the same project", 400)
			return
		}
		rows, err := h.db.QueryContext(r.Context(), "SELECT i.id,d.depends_on_issue_id FROM issues i LEFT JOIN issue_dependencies d ON d.issue_id=i.id WHERE i.project_id=?", projectID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		g := graph.New()
		for rows.Next() {
			var item uint64
			var dependency sql.NullInt64
			if rows.Scan(&item, &dependency) != nil {
				rows.Close()
				http.Error(w, "database error", 500)
				return
			}
			if dependency.Valid {
				if err := g.AddDependency(strconv.FormatUint(item, 10), strconv.FormatInt(dependency.Int64, 10)); err != nil {
					rows.Close()
					http.Error(w, "dependency graph error", 500)
					return
				}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			http.Error(w, "database error", 500)
			return
		}
		rows.Close()
		if err := g.AddDependency(strconv.FormatUint(issueID, 10), strconv.FormatUint(input.DependsOn, 10)); err != nil {
			http.Error(w, "dependency would create a cycle", http.StatusConflict)
			return
		}
		_, err = h.db.ExecContext(r.Context(), "INSERT INTO issue_dependencies(issue_id,depends_on_issue_id) VALUES (?,?)", issueID, input.DependsOn)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		writeIssue(w, 201, map[string]any{"issue_id": issueID, "depends_on_issue_id": input.DependsOn})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *issueHandler) workOrder(w http.ResponseWriter, r *http.Request, uid uint64) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID, err := strconv.ParseUint(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}
	if !h.isMember(r, projectID, uid) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := h.db.QueryContext(r.Context(), "SELECT i.id,d.depends_on_issue_id FROM issues i LEFT JOIN issue_dependencies d ON d.issue_id=i.id WHERE i.project_id=? ORDER BY i.id", projectID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	g := graph.New()
	for rows.Next() {
		var issueID uint64
		var dependency sql.NullInt64
		if err := rows.Scan(&issueID, &dependency); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		id := strconv.FormatUint(issueID, 10)
		g.AddNode(id)
		if dependency.Valid {
			if err := g.AddDependency(id, strconv.FormatInt(dependency.Int64, 10)); err != nil {
				http.Error(w, "dependency graph error", http.StatusInternalServerError)
				return
			}
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		http.Error(w, "dependency graph error", http.StatusInternalServerError)
		return
	}
	ids := make([]uint64, 0, len(order))
	for _, id := range order {
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			http.Error(w, "dependency graph error", http.StatusInternalServerError)
			return
		}
		ids = append(ids, n)
	}
	writeIssue(w, http.StatusOK, map[string]any{"project_id": projectID, "work_order": ids})
}

func (h *issueHandler) isMember(r *http.Request, projectID, userID uint64) bool {
	key := fmt.Sprintf("%d:%d", projectID, userID)
	if h.memberships != nil {
		if cached, ok := h.memberships.Get(key); ok && cached.expires.After(time.Now()) {
			return cached.allowed
		}
	}
	u := fmt.Sprintf("%s/projects/%d/members/check", strings.TrimRight(h.projectURL, "/"), projectID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-User-ID", strconv.FormatUint(userID, 10))
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Member bool `json:"member"`
	}
	isMember := json.NewDecoder(resp.Body).Decode(&result) == nil && result.Member
	if h.memberships != nil {
		h.memberships.Put(key, membershipEntry{allowed: isMember, expires: time.Now().Add(30 * time.Second)})
	}
	return isMember
}
func (h *issueHandler) create(w http.ResponseWriter, r *http.Request, uid uint64) {
	var in issueInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil || in.ProjectID == 0 || strings.TrimSpace(in.Title) == "" || len(in.Title) > 240 {
		http.Error(w, "invalid issue", 400)
		return
	}
	if !h.isMember(r, in.ProjectID, uid) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if in.Status == "" {
		in.Status = "todo"
	}
	if !oneOf(in.Priority, "low", "medium", "high", "critical") || !oneOf(in.Status, "todo", "in_progress", "done") {
		http.Error(w, "invalid priority or status", 400)
		return
	}
	result, err := h.db.ExecContext(r.Context(), "INSERT INTO issues (project_id,title,description,priority,status,assignee_id,reporter_id) VALUES (?,?,?,?,?,?,?)", in.ProjectID, strings.TrimSpace(in.Title), in.Description, in.Priority, in.Status, in.AssigneeID, uid)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	id, _ := result.LastInsertId()
	h.publish(r, "IssueCreated", uint64(id), in.ProjectID, uid, in.Status)
	writeIssue(w, 201, map[string]any{"id": id, "project_id": in.ProjectID, "title": in.Title, "description": in.Description, "priority": in.Priority, "status": in.Status, "assignee_id": in.AssigneeID, "reporter_id": uid})
}
func (h *issueHandler) list(w http.ResponseWriter, r *http.Request, uid uint64) {
	query, err := buildIssueQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	projectID, _ := strconv.ParseUint(r.URL.Query().Get("project_id"), 10, 64)
	if !h.isMember(r, projectID, uid) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := h.db.QueryContext(r.Context(), query.SQL, query.Args...)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	items := []issue{}
	for rows.Next() {
		var x issue
		if err := rows.Scan(&x.ID, &x.ProjectID, &x.Title, &x.Description, &x.Priority, &x.Status, &x.AssigneeID, &x.ReporterID); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		items = append(items, x)
	}
	if rows.Err() != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeIssue(w, 200, items)
}
func (h *issueHandler) item(w http.ResponseWriter, r *http.Request, id uint64) {
	switch r.Method {
	case http.MethodGet:
		var x issue
		err := h.db.QueryRowContext(r.Context(), "SELECT id,project_id,title,description,priority,status,assignee_id,reporter_id FROM issues WHERE id=?", id).Scan(&x.ID, &x.ProjectID, &x.Title, &x.Description, &x.Priority, &x.Status, &x.AssigneeID, &x.ReporterID)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		writeIssue(w, 200, x)
	case http.MethodPut, http.MethodPatch:
		var in issueInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil || strings.TrimSpace(in.Title) == "" {
			http.Error(w, "invalid issue", 400)
			return
		}
		if in.Priority == "" {
			in.Priority = "medium"
		}
		if in.Status == "" {
			in.Status = "todo"
		}
		if !oneOf(in.Priority, "low", "medium", "high", "critical") || !oneOf(in.Status, "todo", "in_progress", "done") {
			http.Error(w, "invalid priority or status", 400)
			return
		}
		var projectID, actorID uint64
		_ = h.db.QueryRowContext(r.Context(), "SELECT project_id FROM issues WHERE id=?", id).Scan(&projectID)
		actorID, _ = strconv.ParseUint(r.Header.Get("X-User-ID"), 10, 64)
		var previousStatus string
		_ = h.db.QueryRowContext(r.Context(), "SELECT status FROM issues WHERE id=?", id).Scan(&previousStatus)
		_, err := h.db.ExecContext(r.Context(), "UPDATE issues SET title=?,description=?,priority=?,status=?,assignee_id=? WHERE id=?", strings.TrimSpace(in.Title), in.Description, in.Priority, in.Status, in.AssigneeID, id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		if previousStatus != in.Status {
			h.publish(r, "StatusChanged", id, projectID, actorID, in.Status)
		}
		writeIssue(w, 200, map[string]bool{"updated": true})
	case http.MethodDelete:
		_, err := h.db.ExecContext(r.Context(), "DELETE FROM issues WHERE id=?", id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *issueHandler) publish(r *http.Request, eventType string, issueID, projectID, actorID uint64, status string) {
	if h.events == nil {
		return
	}
	var eventID [16]byte
	if _, err := rand.Read(eventID[:]); err != nil {
		slog.Warn("generate event ID failed", "error", err)
		return
	}
	body, _ := json.Marshal(map[string]any{"event_id": hex.EncodeToString(eventID[:]), "request_id": r.Header.Get("X-Request-ID"), "issue_id": issueID, "project_id": projectID, "actor_id": actorID, "event_type": eventType, "status": status})
	if err := h.events.Publish("tracker.issue.events", body); err != nil {
		slog.Warn("publish issue event failed", "error", err)
	}
}
func (h *issueHandler) labels(w http.ResponseWriter, r *http.Request, id uint64) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "labels" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := h.db.QueryContext(r.Context(), "SELECT l.id,l.name FROM labels l JOIN issue_labels il ON il.label_id=l.id WHERE il.issue_id=? ORDER BY l.name", id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var labelID uint64
			var name string
			if rows.Scan(&labelID, &name) != nil {
				http.Error(w, "database error", 500)
				return
			}
			out = append(out, map[string]any{"id": labelID, "name": name})
		}
		writeIssue(w, 200, out)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 64 {
		http.Error(w, "invalid label", 400)
		return
	}
	var projectID uint64
	_ = h.db.QueryRowContext(r.Context(), "SELECT project_id FROM issues WHERE id=?", id).Scan(&projectID)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), "INSERT INTO labels (project_id,name) VALUES (?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)", projectID, strings.TrimSpace(input.Name))
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	var labelID uint64
	_ = tx.QueryRowContext(r.Context(), "SELECT id FROM labels WHERE project_id=? AND name=?", projectID, strings.TrimSpace(input.Name)).Scan(&labelID)
	_, err = tx.ExecContext(r.Context(), "INSERT IGNORE INTO issue_labels(issue_id,label_id) VALUES (?,?)", id, labelID)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if tx.Commit() != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeIssue(w, 201, map[string]any{"id": labelID, "name": input.Name})
}
func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
func writeIssue(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
