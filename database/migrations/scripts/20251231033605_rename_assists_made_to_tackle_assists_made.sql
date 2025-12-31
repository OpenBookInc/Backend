-- +goose Up
-- Rename assists_made to tackle_assists_made in nfl_box_scores and nfl_play_statistics
-- to clarify that this stat specifically refers to tackle assists

-- Update nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN assists_made TO tackle_assists_made;

-- Update nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN assists_made TO tackle_assists_made;

-- +goose Down
-- Rollback: Restore original column name

-- Revert nfl_play_statistics table
ALTER TABLE nfl_play_statistics
RENAME COLUMN tackle_assists_made TO assists_made;

-- Revert nfl_box_scores table
ALTER TABLE nfl_box_scores
RENAME COLUMN tackle_assists_made TO assists_made;
