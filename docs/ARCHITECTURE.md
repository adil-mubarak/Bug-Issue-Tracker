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
The gateway readiness endpoint probes all service readiness endpoints, and the
gateway drains requests on SIGINT/SIGTERM with a bounded shutdown period.

Issue list filters are assembled in `services/issue/query.go`: filter values are
SQL parameters, while sort columns and directions come from fixed allowlists.
`after_id` keyset pagination is supported with the default `id` ordering only.
`GET /issues/work-order?project_id=…` returns issue IDs with dependencies before
the issues they block. The Issue and Auth services use their own trie indexes
for project label and authenticated username suggestions; Auth returns local
parts and user IDs rather than email addresses. Project membership calls carry
the gateway request ID and have a three-second client timeout.

Issue events contain random event IDs and request IDs. Activity stores each
event under a unique event ID using an upsert, so duplicate delivery does not
create duplicate activity rows. The Activity service still uses NATS core
subscriptions (not a durable JetStream consumer); prolonged consumer downtime
can therefore lose events.

Compose defaults expose the frontend on port 3000, gateway on 8080, MySQL on
3306, and NATS on 4222. Override published ports in `.env` when those are
already in use. Development passwords are intentionally not production-safe.
