-- +goose Up
-- +goose StatementBegin
ALTER TABLE nfl_plays DROP COLUMN alternative_description;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE nfl_plays ADD COLUMN alternative_description TEXT NOT NULL;
-- +goose StatementEnd
