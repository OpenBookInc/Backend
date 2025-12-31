-- +goose Up
-- Add sack_assists_made column to nfl_box_scores and nfl_play_statistics
-- This stat tracks defensive players who assisted on a sack (similar to tackle assists)

-- Add to nfl_box_scores table
ALTER TABLE nfl_box_scores
ADD COLUMN sack_assists_made NUMERIC(5,1) NOT NULL;

-- Add to nfl_play_statistics table
ALTER TABLE nfl_play_statistics
ADD COLUMN sack_assists_made NUMERIC(5,1) NOT NULL;

-- +goose Down
-- Rollback: Remove sack_assists_made column

-- Remove from nfl_play_statistics table
ALTER TABLE nfl_play_statistics
DROP COLUMN sack_assists_made;

-- Remove from nfl_box_scores table
ALTER TABLE nfl_box_scores
DROP COLUMN sack_assists_made;
