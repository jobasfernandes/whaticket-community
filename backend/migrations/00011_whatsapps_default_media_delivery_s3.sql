-- +goose Up
-- +goose StatementBegin
ALTER TABLE whatsapps ALTER COLUMN media_delivery SET DEFAULT 's3';
UPDATE whatsapps SET media_delivery = 's3' WHERE media_delivery IS NULL OR media_delivery = '' OR media_delivery = 'base64';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE whatsapps ALTER COLUMN media_delivery SET DEFAULT 'base64';
-- +goose StatementEnd
