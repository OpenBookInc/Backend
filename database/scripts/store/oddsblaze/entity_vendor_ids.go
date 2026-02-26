package oddsblaze

import (
	"context"
	"fmt"

	matcher_oddsblaze "github.com/openbook/population-scripts/matcher/oddsblaze"
	"github.com/openbook/population-scripts/store"
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
func UpsertEntityVendorID(s *store.Store, ctx context.Context, e *EntityVendorIDForUpsert) error {
	query := `
		INSERT INTO entity_vendor_ids (entity_type, entity_id, vendor, vendor_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entity_type, entity_id, vendor)
		DO UPDATE SET vendor_id = EXCLUDED.vendor_id
	`

	_, err := s.Pool().Exec(ctx, query, e.EntityType, e.EntityID, e.Vendor, e.VendorID)
	if err != nil {
		return fmt.Errorf("failed to upsert entity vendor ID (type=%s, id=%d, vendor=%s, vendor_id=%s): %w",
			e.EntityType, e.EntityID, e.Vendor, e.VendorID, err)
	}

	return nil
}

// PersistMatchedEntities persists all matched OddsBlaze entity mappings to entity_vendor_ids.
// Returns counts of persisted teams, individuals, and games.
func PersistMatchedEntities(s *store.Store, ctx context.Context, matched *matcher_oddsblaze.MatchedEntities) (teams, individuals, games int, err error) {
	// Persist team mappings
	for _, mt := range matched.Teams {
		if err := UpsertEntityVendorID(s, ctx, &EntityVendorIDForUpsert{
			EntityType: gen.EntityTeam,
			EntityID:   mt.DBTeam.ID,
			Vendor:     gen.VendorOddsBlaze,
			VendorID:   mt.OddsBlazeID,
		}); err != nil {
			return 0, 0, 0, fmt.Errorf("failed to persist team mapping: %w", err)
		}
		teams++
	}

	// Persist individual mappings
	for _, mi := range matched.Individuals {
		if err := UpsertEntityVendorID(s, ctx, &EntityVendorIDForUpsert{
			EntityType: gen.EntityIndividual,
			EntityID:   mi.DBIndividual.ID,
			Vendor:     gen.VendorOddsBlaze,
			VendorID:   mi.OddsBlazeID,
		}); err != nil {
			return 0, 0, 0, fmt.Errorf("failed to persist individual mapping: %w", err)
		}
		individuals++
	}

	// Persist game mappings
	for _, mg := range matched.Games {
		if err := UpsertEntityVendorID(s, ctx, &EntityVendorIDForUpsert{
			EntityType: gen.EntityGame,
			EntityID:   mg.DBGame.ID,
			Vendor:     gen.VendorOddsBlaze,
			VendorID:   mg.OddsBlazeID,
		}); err != nil {
			return 0, 0, 0, fmt.Errorf("failed to persist game mapping: %w", err)
		}
		games++
	}

	return teams, individuals, games, nil
}
