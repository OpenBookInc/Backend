package nba

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Play Statistics Store
// =============================================================================
// Unlike other tables (plays), nba_play_statistics does NOT have a
// unique constraint because a single player can have multiple statistics of
// different types on the same play (e.g., a player could have both a field goal
// and an assist on the same play).
//
// Because of this, we use a DELETE + INSERT approach instead of ON CONFLICT:
// 1. Delete all existing statistics for the given play_id
// 2. Insert all new statistics
//
// This ensures we always have the latest data without complex matching logic,
// and it handles cases where statistics are corrected or removed after review.
// =============================================================================

// NBAPlayStatisticForUpsert represents the data needed to insert a play statistic.
// This is an internal type used only by the store layer for upserts.
// It uses VendorPlayerID instead of IndividualID - the ID is looked up via subquery.
type NBAPlayStatisticForUpsert struct {
	VendorPlayerID         string
	StatType               string // DB enum value as string
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

// ReplaceNBAPlayStatistics replaces all statistics for a play with new statistics.
// This function first deletes all existing statistics for the play, then inserts
// all the new statistics. This is necessary because there is no unique constraint
// on the nba_play_statistics table (a player can have multiple stat entries per play).
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// The VendorPlayerID in each stat is looked up via subquery to get the individual_id.
func ReplaceNBAPlayStatistics(s *store.Store, ctx context.Context, tx pgx.Tx, playID int, stats []*NBAPlayStatisticForUpsert) error {
	// Step 1: Delete all existing statistics for this play
	deleteQuery := `DELETE FROM nba_play_statistics WHERE play_id = $1`
	_, err := tx.Exec(ctx, deleteQuery, playID)
	if err != nil {
		return fmt.Errorf("failed to delete existing statistics for play_id %d: %w", playID, err)
	}

	// Step 2: Insert all new statistics
	// If there are no statistics, we're done (the delete already cleared any old data)
	if len(stats) == 0 {
		return nil
	}

	insertQuery := `
		INSERT INTO nba_play_statistics (
			play_id, individual_id, stat_type,
			two_point_attempts, two_point_makes,
			three_point_attempts, three_point_makes,
			free_throw_attempts, free_throw_makes,
			assists, defensive_rebounds, offensive_rebounds,
			steals, blocks, turnovers_committed, personal_fouls_committed
		)
		VALUES (
			$1,
			(SELECT id FROM individuals WHERE vendor_id = $2),
			$3::nba_stat_type,
			$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	for _, stat := range stats {
		_, err := tx.Exec(ctx, insertQuery,
			playID,
			stat.VendorPlayerID,
			stat.StatType,
			stat.TwoPointAttempts,
			stat.TwoPointMakes,
			stat.ThreePointAttempts,
			stat.ThreePointMakes,
			stat.FreeThrowAttempts,
			stat.FreeThrowMakes,
			stat.Assists,
			stat.DefensiveRebounds,
			stat.OffensiveRebounds,
			stat.Steals,
			stat.Blocks,
			stat.TurnoversCommitted,
			stat.PersonalFoulsCommitted,
		)
		if err != nil {
			return fmt.Errorf("failed to insert statistic for play_id %d, vendor_player_id %s (player may not exist in database): %w",
				playID, stat.VendorPlayerID, err)
		}
	}

	return nil
}
