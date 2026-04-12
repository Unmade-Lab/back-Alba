-- =============================================
-- ONBOARDING & ADVANCED CRM STRUCTURES
-- =============================================

-- 1. Update Workspaces with onboarding status
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS onboarding_completed BOOLEAN DEFAULT FALSE;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS industry VARCHAR(100);
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS onboarding_data JSONB DEFAULT '{}';

-- 2. Pipelines (sales funnels)
CREATE TABLE IF NOT EXISTS pipelines (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    is_default   BOOLEAN DEFAULT FALSE,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipelines_workspace_id ON pipelines(workspace_id);

-- 3. Stages within a pipeline
CREATE TABLE IF NOT EXISTS stages (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_id  UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    sort_order   INT DEFAULT 0,
    probability  INT DEFAULT 100, -- probability of closing from this stage (0-100)
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stages_pipeline_id ON stages(pipeline_id);

-- 4. Departments
CREATE TABLE IF NOT EXISTS departments (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id    UUID REFERENCES departments(id) ON DELETE SET NULL,
    name         VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_departments_workspace_id ON departments(workspace_id);

-- 5. Permissions (Central list of allowed actions)
CREATE TABLE IF NOT EXISTS permissions (
    id          VARCHAR(100) PRIMARY KEY, -- e.g. 'deals:read', 'deals:create'
    description TEXT
);

-- Seed basic permissions
INSERT INTO permissions (id, description) VALUES
('deals:read', 'View sales deals'),
('deals:create', 'Create new sales deals'),
('deals:update', 'Edit sales deals'),
('deals:delete', 'Delete sales deals'),
('companies:read', 'View companies'),
('companies:create', 'Create companies'),
('users:manage', 'Invite and manage other users'),
('settings:update', 'Manage workspace settings')
ON CONFLICT (id) DO NOTHING;

-- 6. Workspace Roles (Custom roles per workspace)
CREATE TABLE IF NOT EXISTS workspace_roles (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL, -- e.g. 'Sales Representative', 'Sales Manager'
    is_system    BOOLEAN DEFAULT FALSE, -- system roles cannot be deleted
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Junction table for Roles <-> Permissions
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id      UUID NOT NULL REFERENCES workspace_roles(id) ON DELETE CASCADE,
    permission_id VARCHAR(100) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Update users to link to a specific modern role (optional for now, but good to have)
ALTER TABLE users ADD COLUMN IF NOT EXISTS workspace_role_id UUID REFERENCES workspace_roles(id) ON DELETE SET NULL;
