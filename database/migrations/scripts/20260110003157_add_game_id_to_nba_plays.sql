-- +goose Up
-- +goose StatementBegin
-- Add game_id column to nba_plays table for associating plays with games
ALTER TABLE nba_plays ADD COLUMN game_id INTEGER NOT NULL REFERENCES games(id);

-- Create index for efficient game-based queries
CREATE INDEX idx_nba_plays_game_id ON nba_plays(game_id);

-- Add unique constraint on (game_id, vendor_id) to support upserts
ALTER TABLE nba_plays ADD CONSTRAINT nba_plays_game_vendor_unique UNIQUE(game_id, vendor_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE nba_plays DROP CONSTRAINT nba_plays_game_vendor_unique;
DROP INDEX idx_nba_plays_game_id;
ALTER TABLE nba_plays DROP COLUMN game_id;
-- +goose StatementEnd
