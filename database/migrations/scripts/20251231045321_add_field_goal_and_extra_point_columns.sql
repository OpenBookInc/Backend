-- +goose Up
-- Add field goal and extra point columns to nfl_box_scores and nfl_play_statistics

-- Add to nfl_box_scores table
ALTER TABLE nfl_box_scores
ADD COLUMN field_goal_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN field_goal_makes NUMERIC(5,1) NOT NULL,
ADD COLUMN field_goal_make_yards NUMERIC(5,1) NOT NULL,
ADD COLUMN extra_point_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN extra_point_makes NUMERIC(5,1) NOT NULL;

-- Add to nfl_play_statistics table
ALTER TABLE nfl_play_statistics
ADD COLUMN field_goal_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN field_goal_makes NUMERIC(5,1) NOT NULL,
ADD COLUMN field_goal_make_yards NUMERIC(5,1) NOT NULL,
ADD COLUMN extra_point_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN extra_point_makes NUMERIC(5,1) NOT NULL;

-- +goose Down
-- Rollback: Remove field goal and extra point columns

-- Remove from nfl_play_statistics table
ALTER TABLE nfl_play_statistics
DROP COLUMN extra_point_makes,
DROP COLUMN extra_point_attempts,
DROP COLUMN field_goal_make_yards,
DROP COLUMN field_goal_makes,
DROP COLUMN field_goal_attempts;

-- Remove from nfl_box_scores table
ALTER TABLE nfl_box_scores
DROP COLUMN extra_point_makes,
DROP COLUMN extra_point_attempts,
DROP COLUMN field_goal_make_yards,
DROP COLUMN field_goal_makes,
DROP COLUMN field_goal_attempts;
