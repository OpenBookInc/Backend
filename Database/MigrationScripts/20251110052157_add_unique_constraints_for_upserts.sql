-- +goose Up
-- Add UNIQUE constraints required for ON CONFLICT upserts

-- leagues: UNIQUE on name (for league upserts by name)
ALTER TABLE leagues ADD CONSTRAINT leagues_name_unique UNIQUE (name);

-- conferences: UNIQUE on vendor_id (for upserts by Sportradar UUID)
ALTER TABLE conferences ADD CONSTRAINT conferences_vendor_id_unique UNIQUE (vendor_id);

-- divisions: UNIQUE on vendor_id (for upserts by Sportradar UUID)
ALTER TABLE divisions ADD CONSTRAINT divisions_vendor_id_unique UNIQUE (vendor_id);

-- teams: UNIQUE on vendor_id (for upserts by Sportradar UUID)
ALTER TABLE teams ADD CONSTRAINT teams_vendor_id_unique UNIQUE (vendor_id);

-- individuals: UNIQUE on vendor_id (for upserts by Sportradar UUID)
ALTER TABLE individuals ADD CONSTRAINT individuals_vendor_id_unique UNIQUE (vendor_id);

-- +goose Down
-- Remove UNIQUE constraints
ALTER TABLE individuals DROP CONSTRAINT IF EXISTS individuals_vendor_id_unique;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_vendor_id_unique;
ALTER TABLE divisions DROP CONSTRAINT IF EXISTS divisions_vendor_id_unique;
ALTER TABLE conferences DROP CONSTRAINT IF EXISTS conferences_vendor_id_unique;
ALTER TABLE leagues DROP CONSTRAINT IF EXISTS leagues_name_unique;
