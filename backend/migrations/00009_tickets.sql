-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tickets (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    unread_messages INTEGER NOT NULL DEFAULT 0,
    last_message TEXT NOT NULL DEFAULT '',
    is_group BOOLEAN NOT NULL DEFAULT false,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    whatsapp_id BIGINT NOT NULL REFERENCES whatsapps(id) ON DELETE CASCADE,
    queue_id BIGINT REFERENCES queues(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tickets_contact_whatsapp_status_idx ON tickets (contact_id, whatsapp_id, status);
CREATE INDEX IF NOT EXISTS tickets_contact_whatsapp_updated_idx ON tickets (contact_id, whatsapp_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS tickets_status_updated_idx ON tickets (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS tickets_user_status_idx ON tickets (user_id, status);
CREATE INDEX IF NOT EXISTS tickets_queue_status_idx ON tickets (queue_id, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS tickets_queue_status_idx;
DROP INDEX IF EXISTS tickets_user_status_idx;
DROP INDEX IF EXISTS tickets_status_updated_idx;
DROP INDEX IF EXISTS tickets_contact_whatsapp_updated_idx;
DROP INDEX IF EXISTS tickets_contact_whatsapp_status_idx;
DROP TABLE IF EXISTS tickets;
-- +goose StatementEnd
