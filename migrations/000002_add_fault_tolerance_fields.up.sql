-- Add fault tolerance and optimistic concurrency control fields to the tasks table

-- First, add nullable columns
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS result TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS error TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS exit_code INTEGER NOT NULL DEFAULT -1;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 5;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS worker_id VARCHAR(255);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMP WITH TIME ZONE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS checkpoint JSONB;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS queued_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS finished_at TIMESTAMP WITH TIME ZONE;

-- Create indexes for improved query performance
CREATE INDEX IF NOT EXISTS idx_tasks_worker_id ON tasks(worker_id);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_last_heartbeat ON tasks(last_heartbeat);

-- Add comment on new columns
COMMENT ON COLUMN tasks.result IS 'Task execution result';
COMMENT ON COLUMN tasks.error IS 'Error message if task failed';
COMMENT ON COLUMN tasks.exit_code IS 'Command exit code (-1 if not yet executed)';
COMMENT ON COLUMN tasks.retry_count IS 'Number of retry attempts made';
COMMENT ON COLUMN tasks.priority IS 'Execution priority (higher numbers = higher priority)';
COMMENT ON COLUMN tasks.worker_id IS 'ID of worker currently executing this task';
COMMENT ON COLUMN tasks.last_heartbeat IS 'Last heartbeat time from worker';
COMMENT ON COLUMN tasks.checkpoint IS 'Checkpoint data for resumable tasks';
COMMENT ON COLUMN tasks.version IS 'Version for optimistic concurrency control';
COMMENT ON COLUMN tasks.queued_at IS 'When the task was queued for execution';
COMMENT ON COLUMN tasks.started_at IS 'When the task started execution';
COMMENT ON COLUMN tasks.finished_at IS 'When the task completed execution';

-- Create a dead letter queue table for permanently failed tasks
CREATE TABLE IF NOT EXISTS dead_letter_tasks (
    id VARCHAR(36) PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    task_data JSONB NOT NULL,
    failure_reason TEXT NOT NULL,
    failed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    retried BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id)
);

-- Create index on dead letter queue
CREATE INDEX IF NOT EXISTS idx_dead_letter_tasks_task_id ON dead_letter_tasks(task_id);
CREATE INDEX IF NOT EXISTS idx_dead_letter_tasks_failed_at ON dead_letter_tasks(failed_at);

-- Add comment to the table
COMMENT ON TABLE dead_letter_tasks IS 'Stores permanently failed tasks for later analysis or retry'; 