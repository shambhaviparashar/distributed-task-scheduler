-- Drop dead letter queue table
DROP TABLE IF EXISTS dead_letter_tasks;

-- Drop indexes
DROP INDEX IF EXISTS idx_tasks_worker_id;
DROP INDEX IF EXISTS idx_tasks_priority;
DROP INDEX IF EXISTS idx_tasks_last_heartbeat;

-- Drop added columns
ALTER TABLE tasks DROP COLUMN IF EXISTS result;
ALTER TABLE tasks DROP COLUMN IF EXISTS error;
ALTER TABLE tasks DROP COLUMN IF EXISTS exit_code;
ALTER TABLE tasks DROP COLUMN IF EXISTS retry_count;
ALTER TABLE tasks DROP COLUMN IF EXISTS priority;
ALTER TABLE tasks DROP COLUMN IF EXISTS worker_id;
ALTER TABLE tasks DROP COLUMN IF EXISTS last_heartbeat;
ALTER TABLE tasks DROP COLUMN IF EXISTS checkpoint;
ALTER TABLE tasks DROP COLUMN IF EXISTS version;
ALTER TABLE tasks DROP COLUMN IF EXISTS queued_at;
ALTER TABLE tasks DROP COLUMN IF EXISTS started_at;
ALTER TABLE tasks DROP COLUMN IF EXISTS finished_at; 