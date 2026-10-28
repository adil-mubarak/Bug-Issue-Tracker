ALTER TABLE activity_logs DROP INDEX activity_event_id_unique;
ALTER TABLE activity_logs ADD UNIQUE KEY activity_event_once (issue_id, event_type, actor_id, occurred_at);
ALTER TABLE activity_logs DROP COLUMN event_id;
