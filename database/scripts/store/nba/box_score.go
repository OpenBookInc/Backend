package nba

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Store
// =============================================================================
// Handles persistence of NBA box scores to the database.
// Box scores are aggregated player statistics for a single game.
//
// Uses ON CONFLICT (game_id, individual_id) for upserts since each player
// can only have one box score per game.
// =============================================================================

// NBABoxScoreForUpsert contains the data needed to upsert a box score
type NBABoxScoreForUpsert struct {
	GameID                 int
	IndividualID           int
	TwoPointAttempts       decimal.Decimal
	TwoPointMakes          decimal.Decimal
	ThreePointAttempts     decimal.Decimal
	ThreePointMakes        decimal.Decimal
	FreeThrowAttempts      decimal.Decimal
	FreeThrowMakes         decimal.Decimal
	Assists                decimal.Decimal
	DefensiveRebounds      decimal.Decimal
	OffensiveRebounds      decimal.Decimal
	Steals                 decimal.Decimal
	Blocks                 decimal.Decimal
	TurnoversCommitted     decimal.Decimal
	PersonalFoulsCommitted decimal.Decimal
}

// UpsertNBABoxScore inserts or updates a box score in the database.
// Uses (game_id, individual_id) as the unique constraint for ON CONFLICT.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func UpsertNBABoxScore(s *store.Store, ctx context.Context, tx pgx.Tx, boxScore *NBABoxScoreForUpsert) error {
	query := `
		INSERT INTO nba_box_scores (
			game_id, individual_id,
			two_point_attempts, two_point_makes,
			three_point_attempts, three_point_makes,
			free_throw_attempts, free_throw_makes,
			assists, defensive_rebounds, offensive_rebounds,
			steals, blocks, turnovers_committed, personal_fouls_committed,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			NOW(), NOW()
		)
		ON CONFLICT (game_id, individual_id)
		DO UPDATE SET
			two_point_attempts = EXCLUDED.two_point_attempts,
			two_point_makes = EXCLUDED.two_point_makes,
			three_point_attempts = EXCLUDED.three_point_attempts,
			three_point_makes = EXCLUDED.three_point_makes,
			free_throw_attempts = EXCLUDED.free_throw_attempts,
			free_throw_makes = EXCLUDED.free_throw_makes,
			assists = EXCLUDED.assists,
			defensive_rebounds = EXCLUDED.defensive_rebounds,
			offensive_rebounds = EXCLUDED.offensive_rebounds,
			steals = EXCLUDED.steals,
			blocks = EXCLUDED.blocks,
			turnovers_committed = EXCLUDED.turnovers_committed,
			personal_fouls_committed = EXCLUDED.personal_fouls_committed,
			updated_at = NOW()
	`

	_, err := tx.Exec(ctx, query,
		boxScore.GameID,
		boxScore.IndividualID,
		boxScore.TwoPointAttempts,
		boxScore.TwoPointMakes,
		boxScore.ThreePointAttempts,
		boxScore.ThreePointMakes,
		boxScore.FreeThrowAttempts,
		boxScore.FreeThrowMakes,
		boxScore.Assists,
		boxScore.DefensiveRebounds,
		boxScore.OffensiveRebounds,
		boxScore.Steals,
		boxScore.Blocks,
		boxScore.TurnoversCommitted,
		boxScore.PersonalFoulsCommitted,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert box score for game_id %d, individual_id %d: %w",
			boxScore.GameID, boxScore.IndividualID, err)
	}

	return nil
}
