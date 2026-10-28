# Issue service guidance

Own issues, labels, filtering, and `issue_db`. Store user IDs, not foreign keys
to another service's database. Use parameterized SQL; dynamic filter values must
never be concatenated into SQL. Use bounded timeouts for service calls.
