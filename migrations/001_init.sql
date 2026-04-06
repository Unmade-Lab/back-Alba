-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================
-- USERS
-- =============================================
CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    name          VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(50)  NOT NULL DEFAULT 'user',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default admin if no users exist
-- password: admin123 (sha256 = 240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9)
INSERT INTO users (id, email, name, password_hash, role)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@alba.local',
    'Administrator',
    '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9',
    'admin'
) ON CONFLICT (id) DO NOTHING;

-- =============================================
-- COMPANIES
-- =============================================
CREATE TABLE IF NOT EXISTS companies (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(255) NOT NULL,
    industry   VARCHAR(100),
    website    VARCHAR(255),
    email      VARCHAR(255),
    phone      VARCHAR(50),
    created_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- DEALS
-- =============================================
CREATE TABLE IF NOT EXISTS deals (
    id          UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255)   NOT NULL,
    amount      NUMERIC(15, 2) NOT NULL DEFAULT 0,
    stage       VARCHAR(100)   NOT NULL DEFAULT 'new',
    company_id  UUID           REFERENCES companies(id) ON DELETE SET NULL,
    owner_id    UUID           REFERENCES users(id)     ON DELETE SET NULL,
    close_date  DATE,
    description TEXT,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- =============================================
-- TASKS
-- =============================================
CREATE TABLE IF NOT EXISTS tasks (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50)  NOT NULL DEFAULT 'pending',
    due_date    TIMESTAMPTZ,
    deal_id     UUID        REFERENCES deals(id) ON DELETE CASCADE,
    assigned_to UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- CHAT MESSAGES
-- =============================================
CREATE TABLE IF NOT EXISTS chat_messages (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id VARCHAR(255) NOT NULL,
    user_id    UUID        REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(20)  NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content    TEXT         NOT NULL,
    widget     JSONB,
    intent     VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id  ON chat_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at  ON chat_messages(created_at DESC);

-- =============================================
-- EVENTS (audit log)
-- =============================================
CREATE TABLE IF NOT EXISTS events (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type  VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id   UUID,
    payload     JSONB,
    user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_entity_id  ON events(entity_id);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
