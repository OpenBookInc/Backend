-- +goose Up
ALTER TYPE individual_status_type ADD VALUE 'unknown';

-- +goose Down
-- PostgreSQL does not support removing enum values directly, so we recreate the type.
-- This will fail if any rows have status = 'unknown' (intentional).

ALTER TYPE individual_status_type RENAME TO individual_status_type_temp;

CREATE TYPE individual_status_type AS ENUM (
    'active',
    'day_to_day',
    'doubtful',
    'out',
    'out_for_season',
    'questionable'
);

ALTER TABLE individual_statuses
    ALTER COLUMN status TYPE individual_status_type
    USING status::text::individual_status_type;

DROP TYPE individual_status_type_temp;
