-- +goose Up
-- Create game_status_type enum
CREATE TYPE game_status_type AS ENUM (
    'scheduled',
    'in_progress',
    'complete',
    'validated',
    'cancelled',
    'postponed',
    'delayed',
    'suspended'
);

-- Create game_statuses table
CREATE TABLE game_statuses (
    game_id INTEGER PRIMARY KEY REFERENCES games(id),
    status game_status_type NOT NULL DEFAULT 'scheduled',
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE game_statuses;
DROP TYPE game_status_type;
