-- +goose Up
-- +goose StatementBegin
ALTER TYPE game_status_type RENAME VALUE 'validated' TO 'closed';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TYPE game_status_type RENAME VALUE 'closed' TO 'validated';
-- +goose StatementEnd
