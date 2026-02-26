package oddsblaze

import (
	"context"
	"fmt"

	matcher_oddsblaze "github.com/openbook/population-scripts/matcher/oddsblaze"
	"github.com/openbook/population-scripts/store"
	"github.com/openbook/shared/models/gen"
)

// PersistMatchedEntities persists all matched OddsBlaze entity mappings to entity_vendor_ids.
// Returns counts of persisted teams, individuals, and games.
func PersistMatchedEntities(s *store.Store, ctx context.Context, matched *matcher_oddsblaze.MatchedEntities) (teams, individuals, games int, err error) {
	// Persist team mappings
	for _, mt := range matched.Teams {
		if err := s.UpsertEntityVendorID(ctx, &store.EntityVendorIDForUpsert{
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
		if err := s.UpsertEntityVendorID(ctx, &store.EntityVendorIDForUpsert{
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
		if err := s.UpsertEntityVendorID(ctx, &store.EntityVendorIDForUpsert{
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
