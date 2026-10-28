# Project service guidance

Own projects, project members, permission checks, and `project_db`. Other
services must call this service's API instead of reading its tables. Use the
shared health handlers and parameterized SQL.
