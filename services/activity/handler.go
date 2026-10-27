package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type activityHandler struct {
	db         *sql.DB
	projectURL string
	client     *http.Client
}

func (h *activityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.ParseUint(r.Header.Get("X-User-ID"), 10, 64)
	if err != nil || uid == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID, err := strconv.ParseUint(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		http.Error(w, "project_id is required", 400)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("%s/projects/%d/members/check", strings.TrimRight(h.projectURL, "/"), projectID), nil)
	if err != nil {
		http.Error(w, "project service unavailable", http.StatusBadGateway)
		return
	}
	req.Header.Set("X-User-ID", strconv.FormatUint(uid, 10))
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "project service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var member struct {
		Member bool `json:"member"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&member) != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !member.Member {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			http.Error(w, "invalid limit", 400)
			return
		}
		limit = n
	}
	rows, err := h.db.QueryContext(r.Context(), "SELECT id,project_id,issue_id,actor_id,event_type,payload,occurred_at FROM activity_logs WHERE project_id=? ORDER BY id DESC LIMIT ?", projectID, limit)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	type item struct {
		ID         uint64          `json:"id"`
		ProjectID  uint64          `json:"project_id"`
		IssueID    uint64          `json:"issue_id"`
		ActorID    uint64          `json:"actor_id"`
		EventType  string          `json:"event_type"`
		Payload    json.RawMessage `json:"payload"`
		OccurredAt string          `json:"occurred_at"`
	}
	items := []item{}
	for rows.Next() {
		var x item
		var at []byte
		if err := rows.Scan(&x.ID, &x.ProjectID, &x.IssueID, &x.ActorID, &x.EventType, &x.Payload, &at); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		x.OccurredAt = string(at)
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
