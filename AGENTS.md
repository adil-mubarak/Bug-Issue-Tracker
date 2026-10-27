# Agent guidance

- Follow the build phases in `Build_Plan_Bug_Issue_Tracker.md` in order.
- Keep services independently owned; never query another service's tables.
- Put shared, generic Go code in `pkg/`, API definitions in `contracts/`, and
  architecture and operational docs in `docs/`.
- Use parameterized SQL and migrations for schema changes.
- Add tests with behavior changes. Run `gofmt`, `go vet ./...`, and `go test ./...`.
- Do not commit secrets; use environment variables and `.env.example`.
