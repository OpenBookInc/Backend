package store

import (
	"context"
	"fmt"
)

// OddsBlazeMarketOutcomeForInsert contains the data needed to insert an OddsBlaze market outcome
type OddsBlazeMarketOutcomeForInsert struct {
	OddsBlazeID string
	Result      string
}

// InsertOddsBlazeMarketOutcome inserts a market outcome into the database.
// Uses INSERT only (no ON CONFLICT) — a duplicate insert indicates a script bug.
func (s *Store) InsertOddsBlazeMarketOutcome(ctx context.Context, o *OddsBlazeMarketOutcomeForInsert) error {
	query := `
		INSERT INTO odds_blaze_market_outcomes (odds_blaze_id, result)
		VALUES ($1, $2::market_outcome_result)
	`

	_, err := s.pool.Exec(ctx, query, o.OddsBlazeID, o.Result)
	if err != nil {
		return fmt.Errorf("failed to insert OddsBlaze market outcome (odds_blaze_id=%s, result=%s): %w",
			o.OddsBlazeID, o.Result, err)
	}

	return nil
}
