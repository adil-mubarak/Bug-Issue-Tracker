# Testing and verification

## Local checks

```sh
make fmt
make lint
make vet
make test                 # Go tests with race detector
make build                # Go packages and Vite production build
docker compose config -q
```

The service tests use `sqlmock` for service-owned SQL and `httptest` servers for
Project membership calls. They cover success, validation, authorization,
gateway JWT routing, event persistence, and duplicate event delivery. They do
not replace tests against an isolated disposable MySQL instance.

## Browser flow

Start the app with `make docker-app-up`, then run:

```sh
cd frontend
npm ci
npx playwright install chromium
PLAYWRIGHT_BASE_URL=http://localhost:3000 npm run test:e2e
```

The Playwright flow registers and logs in a fresh user, creates a project and
issue, edits the issue, checks URL-persisted search/priority filters, changes
status, and checks the auth page at a narrow viewport. It writes test accounts
and data to the configured local databases; use a development environment.

## Seed data

`python3 scripts/seed.py` uses the gateway to idempotently create two demo users,
one project, sample issues and labels. Set `TRACKER_URL` for a non-default
gateway address. Set `SEED_ADMIN_*` and `SEED_MEMBER_*` to change demo credentials.

## Current verification limitations

- Compose and the Playwright run exercise the configured development stack and
  are not substitutes for CI against clean ephemeral databases.
- NATS uses core pub/sub, so events published while Activity is unavailable are
  not durably replayed. Event IDs prevent duplicate rows when events are
  redelivered by a future durable consumer.
- MCP-based MySQL, GitHub, and browser review integrations were unavailable;
  Playwright is run locally via `@playwright/test` instead.
