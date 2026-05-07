-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS whatsapps (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'OPENING',
    qrcode TEXT NOT NULL DEFAULT '',
    session TEXT,
    battery VARCHAR(50),
    plugged BOOLEAN,
    retries INTEGER NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT false,
    greeting_message TEXT NOT NULL DEFAULT '',
    farewell_message TEXT NOT NULL DEFAULT '',
    always_online BOOLEAN NOT NULL DEFAULT false,
    reject_call BOOLEAN NOT NULL DEFAULT false,
    msg_reject_call TEXT NOT NULL DEFAULT '',
    read_messages BOOLEAN NOT NULL DEFAULT false,
    ignore_groups BOOLEAN NOT NULL DEFAULT false,
    ignore_status BOOLEAN NOT NULL DEFAULT false,
    media_delivery VARCHAR(20) NOT NULL DEFAULT 'base64',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS whatsapps_name_uniq ON whatsapps (name);
CREATE UNIQUE INDEX IF NOT EXISTS whatsapps_one_default ON whatsapps (is_default) WHERE is_default = true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS whatsapps_one_default;
DROP INDEX IF EXISTS whatsapps_name_uniq;
DROP TABLE IF EXISTS whatsapps;
-- +goose StatementEnd
