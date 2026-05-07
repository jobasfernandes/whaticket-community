-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS contacts (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL DEFAULT '',
    number VARCHAR(255) NOT NULL DEFAULT '',
    lid VARCHAR(255) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    profile_pic_url TEXT NOT NULL DEFAULT '',
    is_group BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS contacts_number_uniq ON contacts (number) WHERE number != '';
CREATE UNIQUE INDEX IF NOT EXISTS contacts_lid_uniq ON contacts (lid) WHERE lid != '';
CREATE INDEX IF NOT EXISTS contacts_name_lower_idx ON contacts (LOWER(name));

CREATE TABLE IF NOT EXISTS contact_custom_fields (
    id BIGSERIAL PRIMARY KEY,
    contact_id BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ccf_contact_id_idx ON contact_custom_fields (contact_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ccf_contact_id_idx;
DROP TABLE IF EXISTS contact_custom_fields;
DROP INDEX IF EXISTS contacts_name_lower_idx;
DROP INDEX IF EXISTS contacts_lid_uniq;
DROP INDEX IF EXISTS contacts_number_uniq;
DROP TABLE IF EXISTS contacts;
-- +goose StatementEnd
