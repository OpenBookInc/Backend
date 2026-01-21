-- +goose Up
-- +goose StatementBegin
-- Add vendor_deleted column to track soft-deleted records from Sportradar
-- This column is used to mark records that have been removed from the vendor API
-- but we want to preserve in our database for historical purposes.

-- Add to nfl_drives
ALTER TABLE nfl_drives
ADD COLUMN vendor_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE nfl_drives
ALTER COLUMN vendor_deleted DROP DEFAULT;

-- Add to nfl_plays
ALTER TABLE nfl_plays
ADD COLUMN vendor_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE nfl_plays
ALTER COLUMN vendor_deleted DROP DEFAULT;

-- Add to nba_plays
ALTER TABLE nba_plays
ADD COLUMN vendor_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE nba_plays
ALTER COLUMN vendor_deleted DROP DEFAULT;

-- Add to nfl_box_scores
ALTER TABLE nfl_box_scores
ADD COLUMN vendor_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE nfl_box_scores
ALTER COLUMN vendor_deleted DROP DEFAULT;

-- Add to nba_box_scores
ALTER TABLE nba_box_scores
ADD COLUMN vendor_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE nba_box_scores
ALTER COLUMN vendor_deleted DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rollback: Remove vendor_deleted columns from all tables

ALTER TABLE nfl_drives
DROP COLUMN vendor_deleted;

ALTER TABLE nfl_plays
DROP COLUMN vendor_deleted;

ALTER TABLE nba_plays
DROP COLUMN vendor_deleted;

ALTER TABLE nfl_box_scores
DROP COLUMN vendor_deleted;

ALTER TABLE nba_box_scores
DROP COLUMN vendor_deleted;
-- +goose StatementEnd
