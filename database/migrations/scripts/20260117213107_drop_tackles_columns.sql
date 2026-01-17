-- +goose Up
-- +goose StatementBegin
-- Remove tackles_made and tackle_assists_made columns from NFL tables

-- Remove from nfl_play_statistics
ALTER TABLE nfl_play_statistics
DROP COLUMN tackles_made;

ALTER TABLE nfl_play_statistics
DROP COLUMN tackle_assists_made;

-- Remove from nfl_box_scores
ALTER TABLE nfl_box_scores
DROP COLUMN tackles_made;

ALTER TABLE nfl_box_scores
DROP COLUMN tackle_assists_made;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Add tackles_made and tackle_assists_made columns back to NFL tables

-- Add back to nfl_play_statistics
ALTER TABLE nfl_play_statistics
ADD COLUMN tackles_made NUMERIC(5,1) NOT NULL;

ALTER TABLE nfl_play_statistics
ADD COLUMN tackle_assists_made NUMERIC(5,1) NOT NULL;

-- Add back to nfl_box_scores
ALTER TABLE nfl_box_scores
ADD COLUMN tackles_made NUMERIC(5,1) NOT NULL;

ALTER TABLE nfl_box_scores
ADD COLUMN tackle_assists_made NUMERIC(5,1) NOT NULL;
-- +goose StatementEnd
