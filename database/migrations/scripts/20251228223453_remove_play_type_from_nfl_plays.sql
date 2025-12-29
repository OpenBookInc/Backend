-- +goose Up
-- +goose StatementBegin
-- Remove play_type column and nfl_play_type enum from database

-- Remove play_type column from nfl_plays
ALTER TABLE nfl_plays
DROP COLUMN play_type;

-- Drop the nfl_play_type enum
DROP TYPE nfl_play_type;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Recreate nfl_play_type enum and add play_type column back to nfl_plays

-- Recreate the nfl_play_type enum
CREATE TYPE nfl_play_type AS ENUM (
    'pass',
    'rush',
    'punt',
    'kickoff',
    'field_goal',
    'extra_point',
    'two_point_conversion',
    'kneel',
    'spike',
    'penalty'
);

-- Add play_type column back to nfl_plays
ALTER TABLE nfl_plays
ADD COLUMN play_type nfl_play_type NOT NULL;
-- +goose StatementEnd
