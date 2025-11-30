-- +goose Up
-- +goose StatementBegin
ALTER TABLE teams DROP COLUMN IF EXISTS image_url;
ALTER TABLE leagues DROP COLUMN IF EXISTS image_url;
ALTER TABLE sports DROP COLUMN IF EXISTS image_url;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE teams ADD COLUMN image_url TEXT NOT NULL DEFAULT '';
ALTER TABLE leagues ADD COLUMN image_url TEXT NOT NULL DEFAULT '';
ALTER TABLE sports ADD COLUMN image_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
