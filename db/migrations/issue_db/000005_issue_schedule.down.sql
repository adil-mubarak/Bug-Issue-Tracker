ALTER TABLE issues
    DROP INDEX issues_project_start_date,
    DROP INDEX issues_project_due_date,
    DROP COLUMN start_date,
    DROP COLUMN due_date,
    DROP COLUMN assigned_at,
    DROP COLUMN started_at,
    DROP COLUMN completed_at;
