# Project context

This is a Go microservices project with a React/Vite frontend. Implementation is
tracked by `Build_Plan_Bug_Issue_Tracker.md`. MySQL uses one database per
service, with golang-migrate SQL migrations.

Local Compose defaults are for development only. Override `MYSQL_ROOT_PASSWORD`
and `JWT_SECRET` before exposing any service outside a trusted local machine.
Service APIs are routed through the gateway; bearer JWTs are checked there and
the gateway sets `X-User-ID` / `X-User-Role` for internal handlers.

Go unit tests, vet, build, frontend TypeScript/Vite build, Compose parsing,
MySQL health, migrations, and auth service startup were validated. External
MCP-based UI and database review tasks were not run because those MCP servers
are not configured in this workspace. The gateway/auth/project/issue/activity
flow was smoke-tested manually through Compose, including event delivery,
membership permissions, label filtering, and cycle rejection.

Remaining roadmap hardening: add automated service-database integration tests,
browser-level Playwright coverage, a seed-data command, and autocomplete backed
by the trie. The dependency graph, LRU, token bucket, and trie packages have
unit tests; optional heap and Union-Find packages are intentionally omitted.
