package nba

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/store"
	models_nba "github.com/openbook/shared/models/nba"
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
			(SELECT id FROM individuals WHERE sportradar_id = $2),
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

// GetNBAPlayStatisticsByGameID retrieves all statistics for a given game.
// Joins through nba_plays to filter by game_id.
// Only returns statistics where the associated play is not deleted.
// Returns all PlayStatistic records associated with plays in the specified game.
func GetNBAPlayStatisticsByGameID(s *store.Store, ctx context.Context, gameID string) ([]*models_nba.PlayStatistic, error) {
	query := `
		SELECT ps.id, ps.play_id, ps.individual_id, ps.stat_type,
		       ps.two_point_attempts, ps.two_point_makes,
		       ps.three_point_attempts, ps.three_point_makes,
		       ps.free_throw_attempts, ps.free_throw_makes,
		       ps.assists, ps.defensive_rebounds, ps.offensive_rebounds,
		       ps.steals, ps.blocks, ps.turnovers_committed, ps.personal_fouls_committed
		FROM nba_play_statistics ps
		JOIN nba_plays p ON ps.play_id = p.id
		WHERE p.game_id = $1 AND p.vendor_deleted = FALSE
	`

	rows, err := s.Pool().Query(ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query statistics for game_id %s: %w", gameID, err)
	}
	defer rows.Close()

	var stats []*models_nba.PlayStatistic
	for rows.Next() {
		var stat models_nba.PlayStatistic
		err := rows.Scan(
			&stat.ID,
			&stat.PlayID,
			&stat.IndividualID,
			&stat.StatType,
			&stat.TwoPointAttempts,
			&stat.TwoPointMakes,
			&stat.ThreePointAttempts,
			&stat.ThreePointMakes,
			&stat.FreeThrowAttempts,
			&stat.FreeThrowMakes,
			&stat.Assists,
			&stat.DefensiveRebounds,
			&stat.OffensiveRebounds,
			&stat.Steals,
			&stat.Blocks,
			&stat.TurnoversCommitted,
			&stat.PersonalFoulsCommitted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan statistic row: %w", err)
		}
		stats = append(stats, &stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating statistics rows: %w", err)
	}

	return stats, nil
}
