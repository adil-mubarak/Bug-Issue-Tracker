ALTER TABLE projects
    ADD COLUMN deleted_at TIMESTAMP NULL DEFAULT NULL,
    ADD KEY projects_deleted_at (deleted_at, id);
