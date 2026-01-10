-- +goose Up
-- +goose StatementBegin
CREATE TABLE nba_box_scores (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL,
    individual_id INTEGER NOT NULL,
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
    personal_fouls_committed NUMERIC(5,1) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    CONSTRAINT nba_box_scores_game_id_individual_id_key UNIQUE (game_id, individual_id),
    CONSTRAINT nba_box_scores_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(id),
    CONSTRAINT nba_box_scores_individual_id_fkey FOREIGN KEY (individual_id) REFERENCES individuals(id)
);

CREATE INDEX index_nba_box_scores_on_game_id ON nba_box_scores(game_id);
CREATE INDEX index_nba_box_scores_on_individual_id ON nba_box_scores(individual_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS nba_box_scores;
-- +goose StatementEnd
