---
name: db-migration
description: Create and review safe MySQL schema migrations for this monorepo.
---

# Database migration workflow

- Keep each service's schema in its own directory under `db/migrations/<database>`.
- Use the next six-digit golang-migrate sequence and matching `.up.sql` and `.down.sql` files.
- Make migrations safe for MySQL 8.4 and review foreign-key/index behavior.
- Never reference another service's database or create cross-service foreign keys.
- Use parameterized SQL in application code; never put request input into migration SQL.
- Validate with `docker compose up --build` on a fresh volume and verify rollback where practical.
