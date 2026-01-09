-- +goose Up
-- +goose StatementBegin
CREATE TYPE nba_stat_type AS ENUM (
    'field_goal',
    'free_throw',
    'assist',
    'rebound',
    'steal',
    'block',
    'turnover',
    'personal_foul'
);

CREATE TABLE nba_plays (
    id SERIAL PRIMARY KEY,
    vendor_id VARCHAR(255) NOT NULL,
    vendor_sequence NUMERIC(21,1) NOT NULL,
    description TEXT NOT NULL,
    vendor_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    vendor_updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    period_type period_type NOT NULL,
    period_number SMALLINT NOT NULL,
    UNIQUE(vendor_id)
);

CREATE TABLE nba_play_statistics (
    id SERIAL PRIMARY KEY,
    play_id INTEGER NOT NULL REFERENCES nba_plays(id),
    individual_id INTEGER NOT NULL REFERENCES individuals(id),
    stat_type nba_stat_type NOT NULL,
    two_point_attempts NUMERIC(5,1) NOT NULL,
    two_point_makes NUMERIC(5,1) NOT NULL,
    three_point_attempts NUMERIC(5,1) NOT NULL,
    three_point_makes NUMERIC(5,1) NOT NULL,
    free_throw_attempts NUMERIC(5,1) NOT NULL,
    free_throw_makes NUMERIC(5,1) NOT NULL,
    assists NUMERIC(5,1) NOT NULL,
    defensive_rebounds NUMERIC(5,1) NOT NULL,
    offensive_rebounds NUMERIC(5,1) NOT NULL,
    steals NUMERIC(5,1) NOT NULL,
    blocks NUMERIC(5,1) NOT NULL,
    turnovers_committed NUMERIC(5,1) NOT NULL,
    personal_fouls_committed NUMERIC(5,1) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE nba_play_statistics;
DROP TABLE nba_plays;
DROP TYPE nba_stat_type;
-- +goose StatementEnd
