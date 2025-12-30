-- +goose Up

-- Create nfl_box_scores table
CREATE TABLE nfl_box_scores (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id),
    individual_id INTEGER NOT NULL REFERENCES individuals(id),

    -- All stats as NUMERIC(5,1) to handle 0.5 values (split sacks/tackles)
    completions NUMERIC(5,1) NOT NULL,
    incompletions NUMERIC(5,1) NOT NULL,
    receptions NUMERIC(5,1) NOT NULL,
    interceptions NUMERIC(5,1) NOT NULL,
    fumbles NUMERIC(5,1) NOT NULL,
    fumbles_lost NUMERIC(5,1) NOT NULL,
    sacks NUMERIC(5,1) NOT NULL,
    tackles NUMERIC(5,1) NOT NULL,
    assists NUMERIC(5,1) NOT NULL,
    passing_attempts NUMERIC(5,1) NOT NULL,
    rushing_attempts NUMERIC(5,1) NOT NULL,
    receiving_targets NUMERIC(5,1) NOT NULL,
    passing_yards NUMERIC(5,1) NOT NULL,
    rushing_yards NUMERIC(5,1) NOT NULL,
    receiving_yards NUMERIC(5,1) NOT NULL,
    passing_touchdowns NUMERIC(5,1) NOT NULL,
    rushing_touchdowns NUMERIC(5,1) NOT NULL,
    receiving_touchdowns NUMERIC(5,1) NOT NULL,
    interceptions_thrown NUMERIC(5,1) NOT NULL,
    sacks_taken NUMERIC(5,1) NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(game_id, individual_id)
);

CREATE INDEX index_nfl_box_scores_on_game_id ON nfl_box_scores(game_id);
CREATE INDEX index_nfl_box_scores_on_individual_id ON nfl_box_scores(individual_id);

-- +goose Down

DROP TABLE nfl_box_scores;
