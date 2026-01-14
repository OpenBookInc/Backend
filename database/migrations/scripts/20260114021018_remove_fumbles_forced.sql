-- +goose Up
-- +goose StatementBegin
-- Remove fumbles_forced column from NFL tables

-- Remove from nfl_play_statistics
ALTER TABLE nfl_play_statistics
DROP COLUMN fumbles_forced;

-- Remove from nfl_box_scores
ALTER TABLE nfl_box_scores
DROP COLUMN fumbles_forced;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Add fumbles_forced column back to NFL tables

-- Add back to nfl_play_statistics
ALTER TABLE nfl_play_statistics
ADD COLUMN fumbles_forced NUMERIC(5,1) NOT NULL;

-- Add back to nfl_box_scores
ALTER TABLE nfl_box_scores
ADD COLUMN fumbles_forced NUMERIC(5,1) NOT NULL;
-- +goose StatementEnd
