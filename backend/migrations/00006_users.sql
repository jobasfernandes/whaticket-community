-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    profile VARCHAR(50) NOT NULL DEFAULT 'admin',
    whatsapp_id BIGINT REFERENCES whatsapps(id) ON DELETE SET NULL,
    token_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_uniq ON users (email);
CREATE INDEX IF NOT EXISTS users_whatsapp_id_idx ON users (whatsapp_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_whatsapp_id_idx;
DROP INDEX IF EXISTS users_email_uniq;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
