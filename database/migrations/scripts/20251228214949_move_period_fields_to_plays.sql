-- +goose Up
-- +goose StatementBegin
-- Move period_type and period_number from nfl_drives to nfl_plays

-- Add period fields to nfl_plays
ALTER TABLE nfl_plays
ADD COLUMN period_type nfl_period_type NOT NULL,
ADD COLUMN period_number SMALLINT NOT NULL;

-- Remove period fields from nfl_drives
ALTER TABLE nfl_drives
DROP COLUMN period_type,
DROP COLUMN period_number;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Move period_type and period_number back to nfl_drives

-- Add period fields back to nfl_drives
ALTER TABLE nfl_drives
ADD COLUMN period_type nfl_period_type NOT NULL,
ADD COLUMN period_number SMALLINT NOT NULL;

-- Remove period fields from nfl_plays
ALTER TABLE nfl_plays
DROP COLUMN period_type,
DROP COLUMN period_number;
-- +goose StatementEnd
