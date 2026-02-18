-- +goose Up
-- +goose StatementBegin
-- Rename vendor_id to sportradar_id on all entity and play-by-play tables.
-- The vendor_id columns have always stored Sportradar UUIDs; this rename makes that explicit
-- in preparation for adding cross-vendor ID mapping via a new entity_vendor_ids table.

-- Rename vendor_id columns on reference data entity tables
ALTER TABLE teams RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE individuals RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE games RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE conferences RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE divisions RENAME COLUMN vendor_id TO sportradar_id;

-- Rename vendor_id columns on play-by-play tables
ALTER TABLE nfl_drives RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE nfl_plays RENAME COLUMN vendor_id TO sportradar_id;
ALTER TABLE nba_plays RENAME COLUMN vendor_id TO sportradar_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE teams RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE individuals RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE games RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE conferences RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE divisions RENAME COLUMN sportradar_id TO vendor_id;

ALTER TABLE nfl_drives RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE nfl_plays RENAME COLUMN sportradar_id TO vendor_id;
ALTER TABLE nba_plays RENAME COLUMN sportradar_id TO vendor_id;
-- +goose StatementEnd
