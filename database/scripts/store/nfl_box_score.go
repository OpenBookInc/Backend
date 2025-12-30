package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Box Score Store
// =============================================================================
// Handles persistence of NFL box scores to the database.
// Box scores are aggregated player statistics for a single game.
//
// Uses ON CONFLICT (game_id, individual_id) for upserts since each player
// can only have one box score per game.
// =============================================================================

// NFLBoxScoreForUpsert contains the data needed to upsert a box score
type NFLBoxScoreForUpsert struct {
	GameID              int
	IndividualID        int
	Completions         decimal.Decimal
	Incompletions       decimal.Decimal
	Receptions          decimal.Decimal
	Interceptions       decimal.Decimal
	Fumbles             decimal.Decimal
	FumblesLost         decimal.Decimal
	Sacks               decimal.Decimal
	Tackles             decimal.Decimal
	Assists             decimal.Decimal
	PassingAttempts     decimal.Decimal
	RushingAttempts     decimal.Decimal
	ReceivingTargets    decimal.Decimal
	PassingYards        decimal.Decimal
	RushingYards        decimal.Decimal
	ReceivingYards      decimal.Decimal
	PassingTouchdowns   decimal.Decimal
	RushingTouchdowns   decimal.Decimal
	ReceivingTouchdowns decimal.Decimal
	InterceptionsThrown decimal.Decimal
	SacksTaken          decimal.Decimal
}

// UpsertNFLBoxScore inserts or updates a box score in the database.
// Uses (game_id, individual_id) as the unique constraint for ON CONFLICT.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func (s *Store) UpsertNFLBoxScore(ctx context.Context, tx pgx.Tx, boxScore *NFLBoxScoreForUpsert) error {
	query := `
		INSERT INTO nfl_box_scores (
			game_id, individual_id,
			completions, incompletions, receptions,
			interceptions, fumbles, fumbles_lost,
			sacks, tackles, assists,
			passing_attempts, rushing_attempts, receiving_targets,
			passing_yards, rushing_yards, receiving_yards,
			passing_touchdowns, rushing_touchdowns, receiving_touchdowns,
			interceptions_thrown, sacks_taken,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22,
			NOW(), NOW()
		)
		ON CONFLICT (game_id, individual_id)
		DO UPDATE SET
			completions = EXCLUDED.completions,
			incompletions = EXCLUDED.incompletions,
			receptions = EXCLUDED.receptions,
			interceptions = EXCLUDED.interceptions,
			fumbles = EXCLUDED.fumbles,
			fumbles_lost = EXCLUDED.fumbles_lost,
			sacks = EXCLUDED.sacks,
			tackles = EXCLUDED.tackles,
			assists = EXCLUDED.assists,
			passing_attempts = EXCLUDED.passing_attempts,
			rushing_attempts = EXCLUDED.rushing_attempts,
			receiving_targets = EXCLUDED.receiving_targets,
			passing_yards = EXCLUDED.passing_yards,
			rushing_yards = EXCLUDED.rushing_yards,
			receiving_yards = EXCLUDED.receiving_yards,
			passing_touchdowns = EXCLUDED.passing_touchdowns,
			rushing_touchdowns = EXCLUDED.rushing_touchdowns,
			receiving_touchdowns = EXCLUDED.receiving_touchdowns,
			interceptions_thrown = EXCLUDED.interceptions_thrown,
			sacks_taken = EXCLUDED.sacks_taken,
			updated_at = NOW()
	`

	_, err := tx.Exec(ctx, query,
		boxScore.GameID,
		boxScore.IndividualID,
		boxScore.Completions,
		boxScore.Incompletions,
		boxScore.Receptions,
		boxScore.Interceptions,
		boxScore.Fumbles,
		boxScore.FumblesLost,
		boxScore.Sacks,
		boxScore.Tackles,
		boxScore.Assists,
		boxScore.PassingAttempts,
		boxScore.RushingAttempts,
		boxScore.ReceivingTargets,
		boxScore.PassingYards,
		boxScore.RushingYards,
		boxScore.ReceivingYards,
		boxScore.PassingTouchdowns,
		boxScore.RushingTouchdowns,
		boxScore.ReceivingTouchdowns,
		boxScore.InterceptionsThrown,
		boxScore.SacksTaken,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert box score for game_id %d, individual_id %d: %w",
			boxScore.GameID, boxScore.IndividualID, err)
	}

	return nil
}
