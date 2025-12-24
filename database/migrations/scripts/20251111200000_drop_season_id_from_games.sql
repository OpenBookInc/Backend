-- +goose Up
ALTER TABLE games DROP COLUMN season_id;

-- +goose Down
ALTER TABLE games ADD COLUMN season_id BIGINT;
