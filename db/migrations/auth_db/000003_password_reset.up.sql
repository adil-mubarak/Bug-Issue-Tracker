ALTER TABLE users
    ADD COLUMN password_reset_required BOOLEAN NOT NULL DEFAULT FALSE AFTER role,
    ADD COLUMN password_reset_version BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER password_reset_required;
