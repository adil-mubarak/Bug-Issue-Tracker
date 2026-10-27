package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildIssueQueryFiltersAreBoundAndSortIsAllowlisted(t *testing.T) {
	q, err := buildIssueQuery(url.Values{
		"project_id": {"8"}, "status": {"todo"}, "priority": {"high"}, "assignee_id": {"21"},
		"q": {"needle%"}, "label": {"backend, urgent"}, "limit": {"12"},
		"sort": {"created_at"}, "direction": {"asc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q.SQL, "ORDER BY created_at ASC, id DESC LIMIT ?") {
		t.Fatalf("unexpected SQL: %s", q.SQL)
	}
	if strings.Contains(q.SQL, "needle") || strings.Contains(q.SQL, "backend") {
		t.Fatalf("user value interpolated into SQL: %s", q.SQL)
	}
	want := []any{uint64(8), "todo", "high", "21", "%needle%%", "%needle%%", uint64(8), "backend", "urgent", 2, uint64(12)}
	if len(q.Args) != len(want) {
		t.Fatalf("args=%#v", q.Args)
	}
	for i := range want {
		if q.Args[i] != want[i] {
			t.Errorf("arg %d=%#v want %#v", i, q.Args[i], want[i])
		}
	}
}

func TestBuildIssueQueryRejectsUnsafeSortAndMalformedFilters(t *testing.T) {
	for name, values := range map[string]url.Values{
		"sort injection":             {"project_id": {"1"}, "sort": {"id; DROP TABLE issues"}},
		"direction injection":        {"project_id": {"1"}, "direction": {"DESC; --"}},
		"bad cursor":                 {"project_id": {"1"}, "after_id": {"0"}},
		"cursor with alternate sort": {"project_id": {"1"}, "after_id": {"10"}, "sort": {"title"}},
		"empty label":                {"project_id": {"1"}, "label": {"a,,b"}},
		"bad limit":                  {"project_id": {"1"}, "limit": {"-1"}},
		"bad status":                 {"project_id": {"1"}, "status": {"todo OR 1=1"}},
		"bad priority":               {"project_id": {"1"}, "priority": {"severe"}},
		"bad assignee":               {"project_id": {"1"}, "assignee_id": {"not-a-number"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildIssueQuery(values); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
