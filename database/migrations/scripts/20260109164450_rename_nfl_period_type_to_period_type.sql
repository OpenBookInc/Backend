-- +goose Up
-- +goose StatementBegin
ALTER TYPE nfl_period_type RENAME TO period_type;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TYPE period_type RENAME TO nfl_period_type;
-- +goose StatementEnd
