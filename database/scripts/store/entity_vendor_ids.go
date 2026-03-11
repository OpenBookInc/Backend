package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
	"github.com/openbook/shared/models/gen"
	"github.com/openbook/shared/utils"
)

// EntityVendorIDForUpsert contains the data needed to upsert an entity vendor ID mapping
type EntityVendorIDForUpsert struct {
	EntityType gen.Entity
	EntityID   utils.UUID
	Vendor     gen.Vendor
	VendorID   string
}

// UpsertEntityVendorID inserts or updates an entity vendor ID mapping.
// Uses (entity_type, entity_id, vendor) as the unique constraint.
func (s *Store) UpsertEntityVendorID(ctx context.Context, e *EntityVendorIDForUpsert) error {
	query := `
		INSERT INTO entity_vendor_ids (entity_type, entity_id, vendor, vendor_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entity_type, entity_id, vendor)
		DO UPDATE SET vendor_id = EXCLUDED.vendor_id
	`

	_, err := s.pool.Exec(ctx, query, e.EntityType, e.EntityID, e.Vendor, e.VendorID)
	if err != nil {
		return fmt.Errorf("failed to upsert entity vendor ID (type=%s, id=%s, vendor=%s, vendor_id=%s): %w",
			e.EntityType, e.EntityID, e.Vendor, e.VendorID, err)
	}

	return nil
}

// LoadEntityVendorIDs fetches all rows from entity_vendor_ids and registers them in the model registry.
// Entities (teams, individuals, games) should already be loaded in the registry before calling this,
// so that the vendor ID mappings can be resolved.
// Returns the number of vendor IDs loaded.
func (s *Store) LoadEntityVendorIDs(ctx context.Context) (int, error) {
	query := `
		SELECT entity_type, entity_id, vendor, vendor_id
		FROM entity_vendor_ids
		ORDER BY entity_type, entity_id
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query entity_vendor_ids: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var entityType gen.Entity
		var entityID utils.UUID
		var vendor gen.Vendor
		var vendorID string

		if err := rows.Scan(&entityType, &entityID, &vendor, &vendorID); err != nil {
			return count, fmt.Errorf("failed to scan entity_vendor_ids row: %w", err)
		}

		models.Registry.RegisterVendorID(entityType, entityID, vendor, vendorID)
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("error iterating entity_vendor_ids rows: %w", err)
	}

	return count, nil
}
