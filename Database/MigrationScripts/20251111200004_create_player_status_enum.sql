-- +goose Up
CREATE TYPE player_status AS ENUM (
    'Active',
    'Day To Day',
    'Doubtful',
    'Out',
    'Out For Season',
    'Questionable'
);

-- Alter individual_statuses table to use the enum type
ALTER TABLE individual_statuses
    ALTER COLUMN status TYPE player_status USING status::player_status;

-- +goose Down
-- Revert the column back to VARCHAR
ALTER TABLE individual_statuses
    ALTER COLUMN status TYPE VARCHAR(255);

-- Drop the enum type
DROP TYPE player_status;
