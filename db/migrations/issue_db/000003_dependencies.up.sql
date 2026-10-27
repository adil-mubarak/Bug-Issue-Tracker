CREATE TABLE issue_dependencies (
    issue_id BIGINT UNSIGNED NOT NULL,
    depends_on_issue_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, depends_on_issue_id),
    CONSTRAINT issue_dependency_issue_fk FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    CONSTRAINT issue_dependency_target_fk FOREIGN KEY (depends_on_issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    CONSTRAINT issue_dependency_not_self CHECK (issue_id <> depends_on_issue_id)
) ENGINE=InnoDB;
