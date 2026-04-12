-- Migration 009: Tasks table (full schema with safe upgrade)
-- Creates tasks table if not exists, or upgrades existing one.

CREATE TABLE IF NOT EXISTS tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID,
    created_by      UUID,
    assigned_to     UUID,
    entity_id       UUID,
    entity_type     VARCHAR(50),
    title           TEXT NOT NULL,
    description     TEXT,
    due_at          TIMESTAMPTZ,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority        VARCHAR(20) NOT NULL DEFAULT 'medium',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Safe upgrade: add missing columns if table already existed with old schema
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS workspace_id  UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS created_by    UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_to   UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS entity_id     UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS entity_type   VARCHAR(50);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS due_at        TIMESTAMPTZ;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority      VARCHAR(20) NOT NULL DEFAULT 'medium';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS description   TEXT;

-- Create indexes safely
CREATE INDEX IF NOT EXISTS idx_tasks_workspace   ON tasks(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_due_at      ON tasks(due_at);
CREATE INDEX IF NOT EXISTS idx_tasks_status      ON tasks(status);
