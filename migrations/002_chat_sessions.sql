-- =============================================
-- CHAT SESSIONS
-- =============================================
CREATE TABLE IF NOT EXISTS chat_sessions (
    id         VARCHAR(255) PRIMARY KEY,
    user_id    UUID         REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL DEFAULT 'New Chat',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Copy existing sessions from chat_messages so we don't lose data
INSERT INTO chat_sessions (id, user_id, title)
SELECT DISTINCT session_id, user_id, 'Chat ' || TO_CHAR(MIN(created_at), 'YYYY-MM-DD')
FROM chat_messages
WHERE user_id IS NOT NULL
GROUP BY session_id, user_id
ON CONFLICT (id) DO NOTHING;
