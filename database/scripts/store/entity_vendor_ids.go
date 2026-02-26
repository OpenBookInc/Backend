package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models/gen"
)

// EntityVendorIDForUpsert contains the data needed to upsert an entity vendor ID mapping
type EntityVendorIDForUpsert struct {
	EntityType gen.Entity
	EntityID   int
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
		return fmt.Errorf("failed to upsert entity vendor ID (type=%s, id=%d, vendor=%s, vendor_id=%s): %w",
			e.EntityType, e.EntityID, e.Vendor, e.VendorID, err)
	}

	return nil
}
