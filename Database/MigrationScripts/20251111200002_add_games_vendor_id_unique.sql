-- +goose Up
ALTER TABLE games ADD CONSTRAINT games_vendor_id_unique UNIQUE (vendor_id);

-- +goose Down
ALTER TABLE games DROP CONSTRAINT games_vendor_id_unique;
