-- +goose Up
-- +goose StatementBegin
-- Remove field_goal_make_yards column from NFL tables

-- Remove from nfl_play_statistics
ALTER TABLE nfl_play_statistics
DROP COLUMN field_goal_make_yards;

-- Remove from nfl_box_scores
ALTER TABLE nfl_box_scores
DROP COLUMN field_goal_make_yards;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Add field_goal_make_yards column back to NFL tables

-- Add back to nfl_play_statistics
ALTER TABLE nfl_play_statistics
ADD COLUMN field_goal_make_yards NUMERIC(5,1) NOT NULL;

-- Add back to nfl_box_scores
ALTER TABLE nfl_box_scores
ADD COLUMN field_goal_make_yards NUMERIC(5,1) NOT NULL;
-- +goose StatementEnd
