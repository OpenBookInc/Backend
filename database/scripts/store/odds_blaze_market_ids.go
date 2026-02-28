package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models/gen"
)

// OddsBlazeMarketIDForUpsert contains the data needed to upsert an OddsBlaze market ID mapping
type OddsBlazeMarketIDForUpsert struct {
	EntityType  gen.MarketEntity
	EntityID    int
	Sportsbook  gen.Sportsbook
	Side        gen.MarketSide
	OddsBlazeID string
}

// UpsertOddsBlazeMarketID inserts or updates an OddsBlaze market ID mapping.
// Uses (entity_type, entity_id, sportsbook, side) as the unique constraint.
func (s *Store) UpsertOddsBlazeMarketID(ctx context.Context, m *OddsBlazeMarketIDForUpsert) error {
	query := `
		INSERT INTO odds_blaze_market_ids (entity_type, entity_id, sportsbook, side, odds_blaze_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (entity_type, entity_id, sportsbook, side)
		DO UPDATE SET odds_blaze_id = EXCLUDED.odds_blaze_id
	`

	_, err := s.pool.Exec(ctx, query, m.EntityType, m.EntityID, m.Sportsbook, m.Side, m.OddsBlazeID)
	if err != nil {
		return fmt.Errorf("failed to upsert OddsBlaze market ID (entity_type=%s, entity_id=%d, sportsbook=%s, side=%s): %w",
			m.EntityType, m.EntityID, m.Sportsbook, m.Side, err)
	}

	return nil
}
