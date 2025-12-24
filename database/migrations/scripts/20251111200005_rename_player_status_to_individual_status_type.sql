-- +goose Up
ALTER TYPE player_status RENAME TO individual_status_type;

-- +goose Down
ALTER TYPE individual_status_type RENAME TO player_status;
