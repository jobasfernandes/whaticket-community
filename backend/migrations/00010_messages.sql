-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(255) PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    contact_id BIGINT REFERENCES contacts(id) ON DELETE CASCADE,
    body TEXT NOT NULL DEFAULT '',
    media_type VARCHAR(50) NOT NULL DEFAULT 'chat',
    media_url TEXT NOT NULL DEFAULT '',
    from_me BOOLEAN NOT NULL DEFAULT false,
    read BOOLEAN NOT NULL DEFAULT false,
    ack INTEGER NOT NULL DEFAULT 0,
    quoted_msg_id VARCHAR(255) REFERENCES messages(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS messages_ticket_created_idx ON messages (ticket_id, created_at DESC);
CREATE INDEX IF NOT EXISTS messages_quoted_idx ON messages (quoted_msg_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS messages_quoted_idx;
DROP INDEX IF EXISTS messages_ticket_created_idx;
DROP TABLE IF EXISTS messages;
-- +goose StatementEnd
