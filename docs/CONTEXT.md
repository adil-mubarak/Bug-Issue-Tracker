# Trackline project context

Trackline is a Go microservices project with a React/Vite frontend. Implementation is
tracked by `Build_Plan_Bug_Issue_Tracker.md`. MySQL uses one database per
service, with golang-migrate SQL migrations.

Local Compose defaults are for development only. Override `MYSQL_ROOT_PASSWORD`
and `JWT_SECRET` before exposing any service outside a trusted local machine.
Service APIs are routed through the gateway; bearer JWTs are checked there and
the gateway sets `X-User-ID` / `X-User-Role` for internal handlers.

Go tests, golangci-lint, vet, build, frontend TypeScript/Vite build, Compose
parsing and full-stack startup were validated. Migrations report current
versions for all four databases; all service `/health` and `/ready` endpoints
respond successfully. Gateway auth, project and issue routes, and the frontend
HTTP entrypoint were smoke-tested. External
MCP-based UI and database review tasks were not run because those MCP servers
are not configured in this workspace. The gateway/auth/project/issue/activity
flow was smoke-tested manually through Compose, including event delivery,
membership permissions, label filtering, and cycle rejection.

Automated handler tests use SQL mocks and HTTP test servers; there are not yet
container-backed database integration tests. The Playwright smoke flow covers
registration, login, project and issue creation, issue editing, filtering, and
status changes against the Compose gateway. The repeatable API seed script is
`scripts/seed.py`. Issue sorting, work-order,
and label autocomplete and authenticated username autocomplete are implemented
and covered by unit tests. The dependency graph, LRU, token bucket, and trie
packages have unit tests and benchmarks; optional heap and Union-Find packages
are intentionally omitted. A Playwright MCP server was unavailable, so browser
tests run with the local Playwright package instead.
