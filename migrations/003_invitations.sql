-- =============================================
-- INVITATIONS (Employee Onboarding)
-- =============================================
CREATE TABLE IF NOT EXISTS invitations (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    email      VARCHAR(255) UNIQUE NOT NULL,
    name       VARCHAR(255) NOT NULL,
    role       VARCHAR(50)  NOT NULL DEFAULT 'user',
    token      VARCHAR(255) UNIQUE NOT NULL,
    session_id VARCHAR(255), -- Для привязки к сессии чата, из которой создан инвайт
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_invitations_token ON invitations(token);
