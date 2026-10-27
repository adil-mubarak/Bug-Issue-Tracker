# Auth service guidance

Own user registration, login, roles, JWT issuance, and `auth_db`. Never expose
password hashes. Use the shared health handlers and parameterized SQL. Keep
service-specific migrations under `db/migrations/auth_db/`.
