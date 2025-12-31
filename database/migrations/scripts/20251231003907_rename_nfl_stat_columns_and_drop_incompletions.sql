-- +goose Up
-- Rename columns in nfl_box_scores and nfl_play_statistics to clarify defensive stat semantics
-- and drop incompletions column

-- Update nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN completions TO passing_completions;

ALTER TABLE nfl_box_scores
DROP COLUMN incompletions;

ALTER TABLE nfl_box_scores
RENAME COLUMN receptions TO receiving_receptions;

ALTER TABLE nfl_box_scores
RENAME COLUMN interceptions TO interceptions_caught;

ALTER TABLE nfl_box_scores
RENAME COLUMN fumbles TO fumbles_forced;

ALTER TABLE nfl_box_scores
RENAME COLUMN sacks TO sacks_made;

ALTER TABLE nfl_box_scores
RENAME COLUMN tackles TO tackles_made;

ALTER TABLE nfl_box_scores
RENAME COLUMN assists TO assists_made;

-- Update nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN completions TO passing_completions;

ALTER TABLE nfl_play_statistics
DROP COLUMN incompletions;

ALTER TABLE nfl_play_statistics
RENAME COLUMN receptions TO receiving_receptions;

ALTER TABLE nfl_play_statistics
RENAME COLUMN interceptions TO interceptions_caught;

ALTER TABLE nfl_play_statistics
RENAME COLUMN fumbles TO fumbles_forced;

ALTER TABLE nfl_play_statistics
RENAME COLUMN sacks TO sacks_made;

ALTER TABLE nfl_play_statistics
RENAME COLUMN tackles TO tackles_made;

ALTER TABLE nfl_play_statistics
RENAME COLUMN assists TO assists_made;

-- +goose Down
-- Rollback: Restore original column names and add incompletions back

-- Revert nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN assists_made TO assists;

ALTER TABLE nfl_play_statistics
RENAME COLUMN tackles_made TO tackles;

ALTER TABLE nfl_play_statistics
RENAME COLUMN sacks_made TO sacks;

ALTER TABLE nfl_play_statistics
RENAME COLUMN fumbles_forced TO fumbles;

ALTER TABLE nfl_play_statistics
RENAME COLUMN interceptions_caught TO interceptions;

ALTER TABLE nfl_play_statistics
RENAME COLUMN receiving_receptions TO receptions;

ALTER TABLE nfl_play_statistics
ADD COLUMN incompletions NUMERIC(5,1) NOT NULL;

ALTER TABLE nfl_play_statistics
RENAME COLUMN passing_completions TO completions;

-- Revert nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN assists_made TO assists;

ALTER TABLE nfl_box_scores
RENAME COLUMN tackles_made TO tackles;

ALTER TABLE nfl_box_scores
RENAME COLUMN sacks_made TO sacks;

ALTER TABLE nfl_box_scores
RENAME COLUMN fumbles_forced TO fumbles;

ALTER TABLE nfl_box_scores
RENAME COLUMN interceptions_caught TO interceptions;

ALTER TABLE nfl_box_scores
RENAME COLUMN receiving_receptions TO receptions;

ALTER TABLE nfl_box_scores
ADD COLUMN incompletions NUMERIC(5,1) NOT NULL;

ALTER TABLE nfl_box_scores
RENAME COLUMN passing_completions TO completions;
