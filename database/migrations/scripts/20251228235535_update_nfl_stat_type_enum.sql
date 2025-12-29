-- +goose Up
-- +goose StatementBegin
-- Update nfl_stat_type enum to only include the 8 stat types we want to persist

-- Create new enum type with desired values
CREATE TYPE nfl_stat_type_temp AS ENUM (
    'passing',
    'rushing',
    'receiving',
    'defense',
    'fumble',
    'interception',
    'field_goal',
    'extra_point'
);

-- Update the table to use the new enum type
-- Map fumbles -> fumble and interceptions -> interception
ALTER TABLE nfl_play_statistics
ALTER COLUMN stat_type TYPE nfl_stat_type_temp
USING (
    CASE stat_type::text
        WHEN 'fumbles' THEN 'fumble'::nfl_stat_type_temp
        WHEN 'interceptions' THEN 'interception'::nfl_stat_type_temp
        ELSE stat_type::text::nfl_stat_type_temp
    END
);

-- Drop the old enum type
DROP TYPE nfl_stat_type;

-- Rename the new enum type to the original name
ALTER TYPE nfl_stat_type_temp RENAME TO nfl_stat_type;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Recreate original nfl_stat_type enum with all 14 values

-- Create new enum with all original values
CREATE TYPE nfl_stat_type_temp AS ENUM (
    'passing',
    'rushing',
    'receiving',
    'defense',
    'fumbles',
    'interceptions',
    'kick_return',
    'punt_return',
    'kickoff',
    'punting',
    'field_goal',
    'extra_point',
    'penalty',
    'blocked_kick'
);

-- Update the table to use the old enum type
-- Map fumble -> fumbles and interception -> interceptions
ALTER TABLE nfl_play_statistics
ALTER COLUMN stat_type TYPE nfl_stat_type_temp
USING (
    CASE stat_type::text
        WHEN 'fumble' THEN 'fumbles'::nfl_stat_type_temp
        WHEN 'interception' THEN 'interceptions'::nfl_stat_type_temp
        ELSE stat_type::text::nfl_stat_type_temp
    END
);

-- Drop the current enum type
DROP TYPE nfl_stat_type;

-- Rename back to original name
ALTER TYPE nfl_stat_type_temp RENAME TO nfl_stat_type;
-- +goose StatementEnd
