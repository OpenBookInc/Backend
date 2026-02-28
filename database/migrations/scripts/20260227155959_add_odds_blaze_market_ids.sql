-- +goose Up
-- +goose StatementBegin
CREATE TYPE market_entity_type AS ENUM ('nba_market', 'nfl_market');
CREATE TYPE sportsbook_type AS ENUM ('draftkings', 'fanatics');
CREATE TYPE market_side AS ENUM ('over', 'under');

CREATE TABLE odds_blaze_market_ids (
    entity_type market_entity_type NOT NULL,
    entity_id INTEGER NOT NULL,
    sportsbook sportsbook_type NOT NULL,
    side market_side NOT NULL,
    odds_blaze_id VARCHAR(255) NOT NULL,
    CONSTRAINT odds_blaze_market_ids_entity_sportsbook_side_unique
        UNIQUE (entity_type, entity_id, sportsbook, side),
    CONSTRAINT odds_blaze_market_ids_type_sportsbook_id_unique
        UNIQUE (entity_type, sportsbook, odds_blaze_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE odds_blaze_market_ids;
DROP TYPE market_side;
DROP TYPE sportsbook_type;
DROP TYPE market_entity_type;
-- +goose StatementEnd
