ALTER TABLE issues
    ADD COLUMN start_date DATE NULL AFTER position,
    ADD COLUMN due_date DATE NULL AFTER start_date,
    ADD COLUMN assigned_at TIMESTAMP NULL DEFAULT NULL AFTER due_date,
    ADD COLUMN started_at TIMESTAMP NULL DEFAULT NULL AFTER assigned_at,
    ADD COLUMN completed_at TIMESTAMP NULL DEFAULT NULL AFTER started_at,
    ADD KEY issues_project_start_date (project_id, start_date),
    ADD KEY issues_project_due_date (project_id, due_date);
