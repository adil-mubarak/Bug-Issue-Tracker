package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type issueQuery struct {
	SQL   string
	Args  []any
	Limit uint64
}

// buildIssueQuery only interpolates constants from allowlists. All filter values
// remain SQL parameters, including label names and search text.
func buildIssueQuery(values url.Values) (issueQuery, error) {
	projectID, err := strconv.ParseUint(values.Get("project_id"), 10, 64)
	if err != nil || projectID == 0 {
		return issueQuery{}, fmt.Errorf("project_id is required")
	}
	query := issueQuery{SQL: "SELECT id,project_id,title,description,priority,status,assignee_id,reporter_id FROM issues WHERE project_id=?", Args: []any{projectID}, Limit: 50}
	for _, filter := range []struct{ key, column string }{{"status", "status"}, {"priority", "priority"}, {"assignee_id", "assignee_id"}} {
		if value := values.Get(filter.key); value != "" {
			switch filter.key {
			case "status":
				if !oneOf(value, "todo", "in_progress", "done") {
					return issueQuery{}, fmt.Errorf("invalid status")
				}
			case "priority":
				if !oneOf(value, "low", "medium", "high", "critical") {
					return issueQuery{}, fmt.Errorf("invalid priority")
				}
			case "assignee_id":
				id, err := strconv.ParseUint(value, 10, 64)
				if err != nil || id == 0 {
					return issueQuery{}, fmt.Errorf("invalid assignee_id")
				}
			}
			query.SQL += " AND " + filter.column + "=?"
			query.Args = append(query.Args, value)
		}
	}
	if text := strings.TrimSpace(values.Get("q")); text != "" {
		query.SQL += " AND (title LIKE ? OR description LIKE ?)"
		query.Args = append(query.Args, "%"+text+"%", "%"+text+"%")
	}
	if raw := strings.TrimSpace(values.Get("label")); raw != "" {
		labels := strings.Split(raw, ",")
		if len(labels) > 10 {
			return issueQuery{}, fmt.Errorf("too many labels")
		}
		for i := range labels {
			labels[i] = strings.TrimSpace(labels[i])
			if labels[i] == "" {
				return issueQuery{}, fmt.Errorf("empty label")
			}
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(labels)), ",")
		query.SQL += " AND id IN (SELECT il.issue_id FROM issue_labels il JOIN labels l ON l.id=il.label_id WHERE l.project_id=? AND l.name IN (" + placeholders + ") GROUP BY il.issue_id HAVING COUNT(DISTINCT l.name)=?)"
		query.Args = append(query.Args, projectID)
		for _, label := range labels {
			query.Args = append(query.Args, label)
		}
		query.Args = append(query.Args, len(labels))
	}
	if raw := values.Get("after_id"); raw != "" {
		cursor, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || cursor == 0 {
			return issueQuery{}, fmt.Errorf("invalid cursor")
		}
		comparator := "<"
		if strings.EqualFold(values.Get("direction"), "asc") {
			comparator = ">"
		}
		query.SQL += " AND id" + comparator + "?"
		query.Args = append(query.Args, cursor)
	}
	sortColumns := map[string]string{"id": "id", "title": "title", "priority": "priority", "status": "status", "created_at": "created_at"}
	sortBy := values.Get("sort")
	if sortBy == "" {
		sortBy = "id"
	}
	if values.Get("after_id") != "" && sortBy != "id" {
		return issueQuery{}, fmt.Errorf("after_id is only supported with sort=id")
	}
	column, ok := sortColumns[sortBy]
	if !ok {
		return issueQuery{}, fmt.Errorf("unsupported sort field")
	}
	direction := strings.ToUpper(values.Get("direction"))
	if direction == "" {
		direction = "DESC"
	}
	if direction != "ASC" && direction != "DESC" {
		return issueQuery{}, fmt.Errorf("unsupported sort direction")
	}
	query.SQL += " ORDER BY " + column + " " + direction
	if column != "id" {
		query.SQL += ", id DESC"
	}
	if raw := values.Get("limit"); raw != "" {
		limit, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || limit == 0 {
			return issueQuery{}, fmt.Errorf("invalid limit")
		}
		if limit < query.Limit {
			query.Limit = limit
		}
	}
	query.Args = append(query.Args, query.Limit)
	query.SQL += " LIMIT ?"
	return query, nil
}
