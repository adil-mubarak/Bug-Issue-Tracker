# Architecture

The application is a Go monorepo with a gateway and four independently owned
services. Auth, Project, Issue, and Activity each use a separate MySQL database
(`auth_db`, `project_db`, `issue_db`, and `activity_db`). The Compose setup
starts the MySQL and NATS dependencies, applies migrations, and starts all
services plus the static React frontend.

Services communicate through APIs rather than querying another service's
tables. The Issue service calls Project for membership checks and caches the
result briefly in a bounded LRU. Issue-created and status-changed events are
published to NATS for Activity to consume. The gateway is the frontend's single
API entry point: it validates HS256 JWTs and forwards the authenticated user ID
and role to internal services. The frontend proxies `/api/*` to the gateway.

Compose defaults expose the frontend on port 3000, gateway on 8080, MySQL on
3306, and NATS on 4222. Override published ports in `.env` when those are
already in use. Development passwords are intentionally not production-safe.
