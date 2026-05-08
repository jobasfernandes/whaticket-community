-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS quick_answers (
    id BIGSERIAL PRIMARY KEY,
    shortcut VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS quick_answers_shortcut_uniq ON quick_answers (shortcut);
CREATE INDEX IF NOT EXISTS quick_answers_message_lower_idx ON quick_answers (LOWER(message));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS quick_answers_message_lower_idx;
DROP INDEX IF EXISTS quick_answers_shortcut_uniq;
DROP TABLE IF EXISTS quick_answers;
-- +goose StatementEnd
