-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_queues (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    queue_id BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, queue_id)
);

CREATE INDEX IF NOT EXISTS user_queues_queue_id_idx ON user_queues (queue_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS user_queues_queue_id_idx;
DROP TABLE IF EXISTS user_queues;
-- +goose StatementEnd
