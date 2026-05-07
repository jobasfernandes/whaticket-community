-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS whatsapp_queues (
    whatsapp_id BIGINT NOT NULL REFERENCES whatsapps(id) ON DELETE CASCADE,
    queue_id BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (whatsapp_id, queue_id)
);

CREATE INDEX IF NOT EXISTS whatsapp_queues_queue_id_idx ON whatsapp_queues (queue_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS whatsapp_queues_queue_id_idx;
DROP TABLE IF EXISTS whatsapp_queues;
-- +goose StatementEnd
