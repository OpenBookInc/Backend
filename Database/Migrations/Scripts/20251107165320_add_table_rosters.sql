-- +goose Up
CREATE TABLE rosters (
    id SERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL,
    individual_ids BIGINT[] NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS rosters;
