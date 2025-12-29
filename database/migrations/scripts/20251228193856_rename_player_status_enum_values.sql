-- +goose Up
-- Rename individual_status_type enum values to use snake_case
ALTER TYPE individual_status_type RENAME VALUE 'Active' TO 'active';
ALTER TYPE individual_status_type RENAME VALUE 'Day To Day' TO 'day_to_day';
ALTER TYPE individual_status_type RENAME VALUE 'Doubtful' TO 'doubtful';
ALTER TYPE individual_status_type RENAME VALUE 'Out' TO 'out';
ALTER TYPE individual_status_type RENAME VALUE 'Out For Season' TO 'out_for_season';
ALTER TYPE individual_status_type RENAME VALUE 'Questionable' TO 'questionable';

-- +goose Down
-- Revert to original PascalCase/Title Case values
ALTER TYPE individual_status_type RENAME VALUE 'active' TO 'Active';
ALTER TYPE individual_status_type RENAME VALUE 'day_to_day' TO 'Day To Day';
ALTER TYPE individual_status_type RENAME VALUE 'doubtful' TO 'Doubtful';
ALTER TYPE individual_status_type RENAME VALUE 'out' TO 'Out';
ALTER TYPE individual_status_type RENAME VALUE 'out_for_season' TO 'Out For Season';
ALTER TYPE individual_status_type RENAME VALUE 'questionable' TO 'Questionable';
