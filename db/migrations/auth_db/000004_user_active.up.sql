ALTER TABLE users
    ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE AFTER password_reset_version;
