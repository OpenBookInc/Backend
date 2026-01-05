-- +goose Up
-- +goose StatementBegin
ALTER TABLE nfl_drives RENAME COLUMN sequence TO vendor_sequence;
ALTER TABLE nfl_plays RENAME COLUMN sequence TO vendor_sequence;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE nfl_plays RENAME COLUMN vendor_sequence TO sequence;
ALTER TABLE nfl_drives RENAME COLUMN vendor_sequence TO sequence;
-- +goose StatementEnd
