-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS queues (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(20) NOT NULL,
    greeting_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS queues_name_uniq ON queues (name);
CREATE UNIQUE INDEX IF NOT EXISTS queues_color_uniq ON queues (color);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS queues_color_uniq;
DROP INDEX IF EXISTS queues_name_uniq;
DROP TABLE IF EXISTS queues;
-- +goose StatementEnd
