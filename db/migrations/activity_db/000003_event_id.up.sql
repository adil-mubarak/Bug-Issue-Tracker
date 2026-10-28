ALTER TABLE activity_logs ADD COLUMN event_id VARCHAR(64) NULL AFTER id;
UPDATE activity_logs SET event_id = CONCAT('legacy-', id) WHERE event_id IS NULL;
ALTER TABLE activity_logs MODIFY event_id VARCHAR(64) NOT NULL;
ALTER TABLE activity_logs ADD UNIQUE KEY activity_event_id_unique (event_id);
ALTER TABLE activity_logs DROP INDEX activity_event_once;
