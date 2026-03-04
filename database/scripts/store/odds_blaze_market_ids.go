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

// GetUngradedOddsBlazeMarketIDs returns all odds_blaze_ids from odds_blaze_market_ids
// where the associated game has status 'closed' and no outcome has been recorded yet.
// Returns an error if any ungraded markets reference games with no game_statuses entry.
func (s *Store) GetUngradedOddsBlazeMarketIDs(ctx context.Context, marketEntity gen.MarketEntity) ([]string, error) {
	var marketTable string
	switch marketEntity {
	case gen.MarketEntityNbaMarket:
		marketTable = "nba_markets"
	case gen.MarketEntityNflMarket:
		marketTable = "nfl_markets"
	default:
		return nil, fmt.Errorf("unsupported market entity type: %s", marketEntity)
	}

	// Safety check: ensure all games with ungraded markets have a game_statuses entry
	missingGameIDs, err := s.GetGameIDsMissingGameStatus(ctx, marketEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to check for missing game statuses: %w", err)
	}
	if len(missingGameIDs) > 0 {
		return nil, fmt.Errorf("found %d game(s) with ungraded markets but no game_statuses entry: %v — run play-by-play updates for these games before grading", len(missingGameIDs), missingGameIDs)
	}

	query := fmt.Sprintf(`
		SELECT obmi.odds_blaze_id
		FROM odds_blaze_market_ids obmi
		JOIN %s m ON obmi.entity_id = m.id AND obmi.entity_type = $1
		JOIN game_statuses gs ON m.game_id = gs.game_id
		WHERE gs.status = 'closed'
		AND NOT EXISTS (
			SELECT 1 FROM odds_blaze_market_outcomes obmo
			WHERE obmo.odds_blaze_id = obmi.odds_blaze_id
		)
	`, marketTable)

	rows, err := s.pool.Query(ctx, query, string(marketEntity))
	if err != nil {
		return nil, fmt.Errorf("failed to query ungraded OddsBlaze market IDs (entity_type=%s): %w", marketEntity, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan OddsBlaze market ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ungraded OddsBlaze market IDs: %w", err)
	}

	return ids, nil
}

// GetGameIDsMissingGameStatus returns distinct game IDs that have ungraded odds_blaze_market_ids
// entries but no corresponding row in game_statuses. These games need play-by-play updates
// before grading can proceed.
func (s *Store) GetGameIDsMissingGameStatus(ctx context.Context, marketEntity gen.MarketEntity) ([]int, error) {
	var marketTable string
	switch marketEntity {
	case gen.MarketEntityNbaMarket:
		marketTable = "nba_markets"
	case gen.MarketEntityNflMarket:
		marketTable = "nfl_markets"
	default:
		return nil, fmt.Errorf("unsupported market entity type: %s", marketEntity)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT m.game_id
		FROM odds_blaze_market_ids obmi
		JOIN %s m ON obmi.entity_id = m.id AND obmi.entity_type = $1
		WHERE NOT EXISTS (
			SELECT 1 FROM game_statuses gs
			WHERE gs.game_id = m.game_id
		)
		AND NOT EXISTS (
			SELECT 1 FROM odds_blaze_market_outcomes obmo
			WHERE obmo.odds_blaze_id = obmi.odds_blaze_id
		)
	`, marketTable)

	rows, err := s.pool.Query(ctx, query, string(marketEntity))
	if err != nil {
		return nil, fmt.Errorf("failed to query game IDs missing game status (entity_type=%s): %w", marketEntity, err)
	}
	defer rows.Close()

	var gameIDs []int
	for rows.Next() {
		var gameID int
		if err := rows.Scan(&gameID); err != nil {
			return nil, fmt.Errorf("failed to scan game ID: %w", err)
		}
		gameIDs = append(gameIDs, gameID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game IDs missing game status: %w", err)
	}

	return gameIDs, nil
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
