# Agent and skill notes

- Root and service `AGENTS.md` files describe ownership and implementation rules.
- `CLAUDE.md` provides a short project entrypoint for Claude-based tooling.
- `.opencode/skills/db-migration/SKILL.md` documents the migration workflow.
- No MCP-provided tools are assumed by repository code or CI.
- This completion pass used the repository build plan and service ownership
  instructions; database changes use the documented `db-migration` workflow.
- No code-reviewer subagent or Playwright/MySQL MCP was available in this
  session. Tests use `sqlmock` and `httptest` rather than cross-service table
  access.
