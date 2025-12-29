-- +goose Up
-- +goose StatementBegin
-- Restructure nfl_play_statistics to use stat-type-specific columns

-- Add new stat-type-specific columns
ALTER TABLE nfl_play_statistics
ADD COLUMN passing_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN rushing_attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN receiving_targets NUMERIC(5,1) NOT NULL,
ADD COLUMN passing_yards NUMERIC(5,1) NOT NULL,
ADD COLUMN rushing_yards NUMERIC(5,1) NOT NULL,
ADD COLUMN receiving_yards NUMERIC(5,1) NOT NULL,
ADD COLUMN passing_touchdowns NUMERIC(5,1) NOT NULL,
ADD COLUMN rushing_touchdowns NUMERIC(5,1) NOT NULL,
ADD COLUMN receiving_touchdowns NUMERIC(5,1) NOT NULL,
ADD COLUMN interceptions_thrown NUMERIC(5,1) NOT NULL,
ADD COLUMN sacks_taken NUMERIC(5,1) NOT NULL;

-- Drop old generic columns that are no longer needed
ALTER TABLE nfl_play_statistics
DROP COLUMN yards,
DROP COLUMN attempts,
DROP COLUMN targets,
DROP COLUMN touchdowns,
DROP COLUMN first_downs,
DROP COLUMN touchbacks,
DROP COLUMN penalties;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Restore original column structure

-- Add back the old generic columns
ALTER TABLE nfl_play_statistics
ADD COLUMN yards NUMERIC(5,1) NOT NULL,
ADD COLUMN attempts NUMERIC(5,1) NOT NULL,
ADD COLUMN targets NUMERIC(5,1) NOT NULL,
ADD COLUMN touchdowns NUMERIC(5,1) NOT NULL,
ADD COLUMN first_downs NUMERIC(5,1) NOT NULL,
ADD COLUMN touchbacks NUMERIC(5,1) NOT NULL,
ADD COLUMN penalties NUMERIC(5,1) NOT NULL;

-- Drop the new stat-type-specific columns
ALTER TABLE nfl_play_statistics
DROP COLUMN passing_attempts,
DROP COLUMN rushing_attempts,
DROP COLUMN receiving_targets,
DROP COLUMN passing_yards,
DROP COLUMN rushing_yards,
DROP COLUMN receiving_yards,
DROP COLUMN passing_touchdowns,
DROP COLUMN rushing_touchdowns,
DROP COLUMN receiving_touchdowns,
DROP COLUMN interceptions_thrown,
DROP COLUMN sacks_taken;
-- +goose StatementEnd
