CREATE TABLE IF NOT EXISTS notifications (
    id         VARCHAR(36) PRIMARY KEY,
    user_id    VARCHAR(255) NOT NULL,
    channel    VARCHAR(50)  NOT NULL,
    payload    TEXT         NOT NULL,
    status     VARCHAR(50)  NOT NULL DEFAULT 'pending',
    retry_count INT         NOT NULL DEFAULT 0,
    last_error TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
