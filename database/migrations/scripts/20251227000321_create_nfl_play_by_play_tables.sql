-- +goose Up

-- Create enums
CREATE TYPE nfl_period_type AS ENUM (
    'quarter',
    'overtime'
);

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

CREATE TYPE nfl_stat_type AS ENUM (
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

-- Create nfl_drives table
CREATE TABLE nfl_drives (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id),
    vendor_id VARCHAR(255) NOT NULL,
    period_number SMALLINT NOT NULL,
    period_type nfl_period_type NOT NULL,
    sequence NUMERIC(21,1) NOT NULL,
    possession_team_id INTEGER NOT NULL REFERENCES teams(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(game_id, vendor_id)
);

CREATE INDEX index_nfl_drives_on_game_id ON nfl_drives(game_id);

-- Create nfl_plays table
CREATE TABLE nfl_plays (
    id SERIAL PRIMARY KEY,
    drive_id INTEGER NOT NULL REFERENCES nfl_drives(id),
    vendor_id VARCHAR(255) NOT NULL,
    play_type nfl_play_type NOT NULL,
    sequence NUMERIC(21,1) NOT NULL,
    description TEXT NOT NULL,
    alternative_description TEXT NOT NULL,
    nullified BOOLEAN NOT NULL,
    vendor_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    vendor_updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(drive_id, vendor_id)
);

CREATE INDEX index_nfl_plays_on_drive_id ON nfl_plays(drive_id);

-- Create nfl_play_statistics table
CREATE TABLE nfl_play_statistics (
    id SERIAL PRIMARY KEY,
    play_id INTEGER NOT NULL REFERENCES nfl_plays(id),
    individual_id INTEGER NOT NULL REFERENCES individuals(id),
    stat_type nfl_stat_type NOT NULL,

    -- All stats as NUMERIC(5,1) to handle 0.5 values (split sacks/tackles)
    attempts NUMERIC(5,1) NOT NULL,
    yards NUMERIC(5,1) NOT NULL,
    completions NUMERIC(5,1) NOT NULL,
    incompletions NUMERIC(5,1) NOT NULL,
    receptions NUMERIC(5,1) NOT NULL,
    targets NUMERIC(5,1) NOT NULL,
    touchdowns NUMERIC(5,1) NOT NULL,
    first_downs NUMERIC(5,1) NOT NULL,
    interceptions NUMERIC(5,1) NOT NULL,
    fumbles NUMERIC(5,1) NOT NULL,
    fumbles_lost NUMERIC(5,1) NOT NULL,
    sacks NUMERIC(5,1) NOT NULL,
    tackles NUMERIC(5,1) NOT NULL,
    assists NUMERIC(5,1) NOT NULL,
    touchbacks NUMERIC(5,1) NOT NULL,
    penalties NUMERIC(5,1) NOT NULL,

    nullified BOOLEAN NOT NULL
);

CREATE INDEX index_nfl_play_statistics_on_play_id ON nfl_play_statistics(play_id);
CREATE INDEX index_nfl_play_statistics_on_individual_id ON nfl_play_statistics(individual_id);

-- +goose Down

-- Drop tables in reverse order (due to foreign key constraints)
DROP TABLE nfl_play_statistics;
DROP TABLE nfl_plays;
DROP TABLE nfl_drives;

-- Drop enums
DROP TYPE nfl_stat_type;
DROP TYPE nfl_play_type;
DROP TYPE nfl_period_type;
