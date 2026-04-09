-- =============================================
-- WORKSPACES (Multi-Tenancy)
-- =============================================
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Создаем дефолтный workspace для уже существующего админа (если он есть)
INSERT INTO workspaces (id, name) 
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Workspace') 
ON CONFLICT DO NOTHING;

-- 1. USERS
ALTER TABLE users ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE users SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE users ALTER COLUMN workspace_id SET NOT NULL;

-- 2. COMPANIES
ALTER TABLE companies ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE companies SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE companies ALTER COLUMN workspace_id SET NOT NULL;

-- 3. DEALS
ALTER TABLE deals ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE deals SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE deals ALTER COLUMN workspace_id SET NOT NULL;

-- 4. TASKS
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE tasks SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE tasks ALTER COLUMN workspace_id SET NOT NULL;

-- 5. CHAT_SESSIONS
ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE chat_sessions SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE chat_sessions ALTER COLUMN workspace_id SET NOT NULL;

-- 6. CHAT_MESSAGES
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE chat_messages SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE chat_messages ALTER COLUMN workspace_id SET NOT NULL;

-- 7. EVENTS
ALTER TABLE events ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE events SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE events ALTER COLUMN workspace_id SET NOT NULL;

-- 8. INVITATIONS
ALTER TABLE invitations ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE invitations SET workspace_id = '00000000-0000-0000-0000-000000000001' WHERE workspace_id IS NULL;
ALTER TABLE invitations ALTER COLUMN workspace_id SET NOT NULL;

-- ИНДЕКСЫ ДЛЯ УСКОРЕНИЯ ВЫБОРКИ ПО WORKSPACE
CREATE INDEX IF NOT EXISTS idx_users_workspace_id ON users(workspace_id);
CREATE INDEX IF NOT EXISTS idx_companies_workspace_id ON companies(workspace_id);
CREATE INDEX IF NOT EXISTS idx_deals_workspace_id ON deals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tasks_workspace_id ON tasks(workspace_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_workspace_id ON chat_sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_workspace_id ON chat_messages(workspace_id);
CREATE INDEX IF NOT EXISTS idx_events_workspace_id ON events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_invitations_workspace_id ON invitations(workspace_id);
