# Database migrations

Each service owns a database and its own golang-migrate directory. Add paired
`NNNNNN_description.up.sql` and `.down.sql` files to the relevant directory.
Compose runs migrations after MySQL is healthy. `000001_initial` is a no-op
baseline retained for migration history; each service's schema is introduced
by its subsequent numbered migrations alongside the corresponding service
implementation.

To apply a service migration manually from the repository root:

```sh
migrate -path db/migrations/auth_db \
  -database 'mysql://root:<MYSQL_ROOT_PASSWORD>@tcp(localhost:3306)/auth_db?multiStatements=true' up
```
