-- +goose Up
-- +goose StatementBegin

-- Add venue fields to teams table
ALTER TABLE teams ADD COLUMN venue_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN venue_city VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN venue_state VARCHAR(50) NOT NULL DEFAULT '';

-- Add player fields to individuals table
ALTER TABLE individuals ADD COLUMN position VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE individuals ADD COLUMN jersey_number VARCHAR(10) NOT NULL DEFAULT '';

-- Add alias to conferences and divisions tables
ALTER TABLE conferences ADD COLUMN alias VARCHAR(10) NOT NULL DEFAULT '';
ALTER TABLE divisions ADD COLUMN alias VARCHAR(10) NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove venue fields from teams table
ALTER TABLE teams DROP COLUMN IF EXISTS venue_name;
ALTER TABLE teams DROP COLUMN IF EXISTS venue_city;
ALTER TABLE teams DROP COLUMN IF EXISTS venue_state;

-- Remove player fields from individuals table
ALTER TABLE individuals DROP COLUMN IF EXISTS position;
ALTER TABLE individuals DROP COLUMN IF EXISTS jersey_number;

-- Remove alias from conferences and divisions tables
ALTER TABLE conferences DROP COLUMN IF EXISTS alias;
ALTER TABLE divisions DROP COLUMN IF EXISTS alias;

-- +goose StatementEnd
