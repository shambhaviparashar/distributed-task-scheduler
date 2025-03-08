CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    command TEXT NOT NULL,
    schedule VARCHAR(100), -- Cron expression
    retries INTEGER NOT NULL DEFAULT 3,
    timeout INTEGER NOT NULL DEFAULT 300, -- In seconds
    dependencies TEXT[], -- Array of task IDs
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);

-- Add a comment to the table
COMMENT ON TABLE tasks IS 'Stores task definitions and their execution status'; 