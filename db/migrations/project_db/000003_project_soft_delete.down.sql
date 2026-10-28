ALTER TABLE projects
    DROP KEY projects_deleted_at,
    DROP COLUMN deleted_at;
