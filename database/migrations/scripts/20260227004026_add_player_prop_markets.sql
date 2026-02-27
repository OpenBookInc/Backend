-- +goose Up
-- +goose StatementBegin
CREATE TYPE nba_player_prop_type AS ENUM (
    'assists',
    'blocks',
    'blocks_steals',
    'double_double',
    'points',
    'points_assists',
    'points_assists_rebounds',
    'points_rebounds',
    'rebounds',
    'rebounds_assists',
    'steals',
    'three_point_makes',
    'triple_double'
);

CREATE TYPE nfl_player_prop_type AS ENUM (
    'extra_point_makes',
    'field_goal_makes',
    'interceptions_thrown',
    'kicking_points',
    'passing_attempts',
    'passing_completions',
    'passing_plus_rushing_yards',
    'passing_touchdowns',
    'passing_yards',
    'receiving_receptions',
    'receiving_yards',
    'rushing_attempts',
    'rushing_plus_receiving_yards',
    'rushing_yards',
    'sacks_made'
);

CREATE TABLE nba_markets (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id),
    individual_id INTEGER NOT NULL REFERENCES individuals(id),
    market_type nba_player_prop_type NOT NULL,
    market_line DECIMAL(5,1) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    CONSTRAINT nba_markets_game_individual_type_line_unique
        UNIQUE (game_id, individual_id, market_type, market_line)
);

CREATE INDEX idx_nba_markets_game_id ON nba_markets (game_id);
CREATE INDEX idx_nba_markets_individual_id ON nba_markets (individual_id);
CREATE INDEX idx_nba_markets_game_individual ON nba_markets (game_id, individual_id);

CREATE TABLE nfl_markets (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id),
    individual_id INTEGER NOT NULL REFERENCES individuals(id),
    market_type nfl_player_prop_type NOT NULL,
    market_line DECIMAL(5,1) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    CONSTRAINT nfl_markets_game_individual_type_line_unique
        UNIQUE (game_id, individual_id, market_type, market_line)
);

CREATE INDEX idx_nfl_markets_game_id ON nfl_markets (game_id);
CREATE INDEX idx_nfl_markets_individual_id ON nfl_markets (individual_id);
CREATE INDEX idx_nfl_markets_game_individual ON nfl_markets (game_id, individual_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE nfl_markets;
DROP TABLE nba_markets;
DROP TYPE nfl_player_prop_type;
DROP TYPE nba_player_prop_type;
-- +goose StatementEnd
