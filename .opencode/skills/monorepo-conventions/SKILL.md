---
name: monorepo-conventions
description: Use when writing, reviewing, testing, or documenting code in this monorepo, including MySQL migrations, Go services, React frontend, Redis usage, Docker Compose, API contracts, and service boundaries.
---

# Monorepo conventions

## Service boundaries
- Each service owns its code, database, and Redis keys; no service reads or writes another's.
- Services communicate only through APIs or events, never through shared databases or shared Redis keys.
- Shared code goes in a dedicated shared module and must stay free of service-specific logic.

## Database migrations (MySQL 8.4)
- Keep each service's schema in its own directory under `db/migrations/<database>`.
- Use the next six-digit golang-migrate sequence and matching `.up.sql` and `.down.sql` files.
- Never edit a migration that has already been applied; add a new one instead.
- Write `.down.sql` to reverse the changes in the opposite order.
- Use `utf8mb4`, explicit index names, and review foreign-key and index behavior.
- Never reference another service's database or create cross-service foreign keys.
- Never put request input into migration SQL.
- Validate with `docker compose up --build` on a fresh volume and verify rollback where practical.

## Go services
- Keep each service in its own directory and follow the repository's Go module layout; do not import another service's `internal/` packages. This repository currently uses a single root `go.mod`.
- Pass `context.Context` as the first parameter of any function that does I/O, and honor cancellation.
- Wrap errors with `fmt.Errorf("doing X: %w", err)`; never ignore an error or both log and return it.
- Load config from environment variables at startup and fail fast if required values are missing.
- Use `database/sql` with parameterized queries only; never build SQL from request input.
- Shut down gracefully on SIGTERM: stop accepting requests, drain in-flight work, close DB and Redis connections.
- Write table-driven tests; run `go vet ./...` and `go test -race ./...` before reporting PASS.

## React frontend
- Use function components and hooks; keep components small and single-purpose.
- Put API calls in a dedicated client layer, not inside components.
- Handle loading, error, and empty states for every data fetch.
- Never store secrets or tokens in source code; follow the project's existing token-storage approach.
- Make interactive elements keyboard-accessible and use semantic HTML and labels.
- Add component tests for new behavior; run lint, type checks, and tests before reporting PASS.

## Redis
- Prefix every key with the service name, such as `orders:session:<id>`.
- Set a TTL on every key.
- Use `SCAN`, never `KEYS`.
- Treat Redis as a cache that can be lost, not as the source of truth.
- Never read or write another service's keys.

## Docker and Compose
- Use multi-stage Dockerfiles and run containers as a non-root user.
- Pin base image versions; do not use `latest`.
- Add healthchecks and use `depends_on` with `condition: service_healthy`.
- Pass config through environment variables; never bake secrets into images or commit them in compose files.
- Use named volumes for database data and keep `.dockerignore` files up to date.

## API contracts
- Follow the existing REST or gRPC conventions; document every endpoint in the OpenAPI spec or proto files.
- Use one consistent error response format and a consistent pagination style.
- Make backward-compatible changes only; version the API for breaking changes.

## Testing
- Run the checks for every area you changed before reporting PASS: Go tests with `-race`, frontend lint and tests, and a Compose build when Docker or migrations changed.
- Prefer integration tests against real MySQL and Redis in containers for data-layer changes.
- Record exact commands and outcomes; separate test failures from environment problems.

## Security
- Validate and sanitize all external input at the service boundary.
- Enforce authentication and authorization on every non-public endpoint.
- Never log secrets, tokens, or personal data.
- Keep CORS rules restrictive and check new dependencies for known vulnerabilities.
