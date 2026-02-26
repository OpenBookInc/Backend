-- +goose Up
-- +goose StatementBegin
-- Create enums for use in entity_vendor_ids
CREATE TYPE entity_type AS ENUM ('team', 'individual', 'game');
CREATE TYPE vendor_type AS ENUM ('sportradar', 'odds_blaze');

-- Create cross-vendor ID mapping table.
-- Stores mappings between internal entity IDs and external vendor IDs for any vendor
-- (e.g., OddsBlaze). Sportradar IDs remain on the entity tables as sportradar_id.
CREATE TABLE entity_vendor_ids (
    entity_type entity_type  NOT NULL,
    entity_id   INTEGER      NOT NULL,
    vendor      vendor_type  NOT NULL,
    vendor_id   VARCHAR(255) NOT NULL,
    PRIMARY KEY (entity_type, entity_id, vendor),
    CONSTRAINT entity_vendor_ids_type_vendor_id_unique UNIQUE (entity_type, vendor, vendor_id)
);

CREATE INDEX idx_entity_vendor_ids_type_entity ON entity_vendor_ids (entity_type, entity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE entity_vendor_ids;
DROP TYPE vendor_type;
DROP TYPE entity_type;
-- +goose StatementEnd
