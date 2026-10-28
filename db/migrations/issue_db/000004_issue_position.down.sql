ALTER TABLE issues
    DROP INDEX issues_project_status_position,
    DROP COLUMN position;
