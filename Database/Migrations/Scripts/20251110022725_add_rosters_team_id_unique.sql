-- +goose Up
ALTER TABLE rosters ADD CONSTRAINT rosters_team_id_unique UNIQUE (team_id);

-- +goose Down
ALTER TABLE rosters DROP CONSTRAINT IF EXISTS rosters_team_id_unique;
