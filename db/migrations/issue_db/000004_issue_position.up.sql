ALTER TABLE issues
    ADD COLUMN position BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER status,
    ADD KEY issues_project_status_position (project_id, status, position);

-- Preserve the existing ID order; new issues use their monotonically increasing
-- ID as an append position until a column is explicitly reordered.
UPDATE issues SET position = id;
