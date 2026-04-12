-- Migration 009: Tasks table
-- Adds task management for the CRM, linked to users, deals, and companies.

CREATE TABLE IF NOT EXISTS tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    assigned_to     UUID REFERENCES users(id) ON DELETE SET NULL,
    entity_id       UUID,          -- Optional FK to deal or company (polymorphic)
    entity_type     VARCHAR(50),   -- 'deal', 'company', 'contact', etc.
    title           TEXT NOT NULL,
    description     TEXT,
    due_at          TIMESTAMPTZ,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, in_progress, completed, cancelled
    priority        VARCHAR(20) NOT NULL DEFAULT 'medium',   -- low, medium, high, urgent
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_workspace   ON tasks(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_due_at      ON tasks(due_at);
CREATE INDEX IF NOT EXISTS idx_tasks_status      ON tasks(status);
