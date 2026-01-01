-- +goose Up
-- Rename fumbles_lost to fumbles_committed in nfl_box_scores and nfl_play_statistics
-- to clarify that this stat represents fumbles committed by a player (not necessarily lost)

-- Update nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN fumbles_lost TO fumbles_committed;

-- Update nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN fumbles_lost TO fumbles_committed;

-- +goose Down
-- Rollback: Restore original column name

-- Revert nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN fumbles_committed TO fumbles_lost;

-- Revert nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN fumbles_committed TO fumbles_lost;
