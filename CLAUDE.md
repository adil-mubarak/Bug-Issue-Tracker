# Repository guidance

Read `AGENTS.md` and `Build_Plan_Bug_Issue_Tracker.md` before making changes.
Implement phases in order and keep each service's data private to that service.
Run `gofmt`, `go vet ./...`, and `go test ./...` for Go changes. Database
changes must use paired, numbered golang-migrate up/down files.
