package nfl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Play Statistics Store
// =============================================================================
// Unlike other tables (drives, plays), nfl_play_statistics does NOT have a
// unique constraint because a single player can have multiple statistics of
// different types on the same play (e.g., a player could have both a reception
// and a fumble on the same play).
//
// Because of this, we use a DELETE + INSERT approach instead of ON CONFLICT:
// 1. Delete all existing statistics for the given play_id
// 2. Insert all new statistics
//
// This ensures we always have the latest data without complex matching logic,
// and it handles cases where statistics are corrected or removed after review.
// =============================================================================

// PlayStatisticForUpsert represents the data needed to insert a play statistic.
// This is an internal type used only by the store layer for upserts.
// It uses VendorPlayerID instead of IndividualID - the ID is looked up via subquery.
type PlayStatisticForUpsert struct {
	VendorPlayerID      string
	StatType            string
	PassingAttempts     decimal.Decimal
	RushingAttempts     decimal.Decimal
	ReceivingTargets    decimal.Decimal
	PassingYards        decimal.Decimal
	RushingYards        decimal.Decimal
	ReceivingYards      decimal.Decimal
	PassingTouchdowns   decimal.Decimal
	RushingTouchdowns   decimal.Decimal
	ReceivingTouchdowns decimal.Decimal
	PassingCompletions  decimal.Decimal
	ReceivingReceptions decimal.Decimal
	InterceptionsThrown decimal.Decimal
	InterceptionsCaught decimal.Decimal
	FumblesCommitted    decimal.Decimal
	SacksTaken          decimal.Decimal
	SacksMade           decimal.Decimal
	SackAssistsMade     decimal.Decimal
	FieldGoalAttempts   decimal.Decimal
	FieldGoalMakes      decimal.Decimal
	ExtraPointAttempts  decimal.Decimal
	ExtraPointMakes     decimal.Decimal
	Nullified           bool
}

// ReplaceNFLPlayStatistics replaces all statistics for a play with new statistics.
// This function first deletes all existing statistics for the play, then inserts
// all the new statistics. This is necessary because there is no unique constraint
// on the nfl_play_statistics table (a player can have multiple stat entries per play).
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// The VendorPlayerID in each stat is looked up via subquery to get the individual_id.
func ReplaceNFLPlayStatistics(s *store.Store, ctx context.Context, tx pgx.Tx, playID int, stats []*PlayStatisticForUpsert) error {
	// Step 1: Delete all existing statistics for this play
	deleteQuery := `DELETE FROM nfl_play_statistics WHERE play_id = $1`
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
		INSERT INTO nfl_play_statistics (
			play_id, individual_id, stat_type,
			passing_attempts, rushing_attempts, receiving_targets,
			passing_yards, rushing_yards, receiving_yards,
			passing_touchdowns, rushing_touchdowns, receiving_touchdowns,
			passing_completions, receiving_receptions,
			interceptions_thrown, interceptions_caught,
			fumbles_committed,
			sacks_taken, sacks_made, sack_assists_made,
			field_goal_attempts, field_goal_makes,
			extra_point_attempts, extra_point_makes,
			nullified
		)
		VALUES (
			$1,
			(SELECT id FROM individuals WHERE sportradar_id = $2),
			$3::nfl_stat_type,
			$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25
		)
	`

	for _, stat := range stats {
		_, err := tx.Exec(ctx, insertQuery,
			playID,
			stat.VendorPlayerID,
			stat.StatType,
			stat.PassingAttempts,
			stat.RushingAttempts,
			stat.ReceivingTargets,
			stat.PassingYards,
			stat.RushingYards,
			stat.ReceivingYards,
			stat.PassingTouchdowns,
			stat.RushingTouchdowns,
			stat.ReceivingTouchdowns,
			stat.PassingCompletions,
			stat.ReceivingReceptions,
			stat.InterceptionsThrown,
			stat.InterceptionsCaught,
			stat.FumblesCommitted,
			stat.SacksTaken,
			stat.SacksMade,
			stat.SackAssistsMade,
			stat.FieldGoalAttempts,
			stat.FieldGoalMakes,
			stat.ExtraPointAttempts,
			stat.ExtraPointMakes,
			stat.Nullified,
		)
		if err != nil {
			return fmt.Errorf("failed to insert statistic for play_id %d, vendor_player_id %s (player may not exist in database): %w",
				playID, stat.VendorPlayerID, err)
		}
	}

	return nil
}

// GetNFLPlayStatisticsByPlayID retrieves all statistics for a given play.
// Only returns statistics where the associated play and drive are not deleted.
func GetNFLPlayStatisticsByPlayID(s *store.Store, ctx context.Context, playID int) ([]*models_nfl.PlayStatistic, error) {
	query := `
		SELECT ps.id, ps.play_id, ps.individual_id, ps.stat_type,
		       ps.passing_attempts, ps.rushing_attempts, ps.receiving_targets,
		       ps.passing_yards, ps.rushing_yards, ps.receiving_yards,
		       ps.passing_touchdowns, ps.rushing_touchdowns, ps.receiving_touchdowns,
		       ps.passing_completions, ps.receiving_receptions,
		       ps.interceptions_thrown, ps.interceptions_caught,
		       ps.fumbles_committed,
		       ps.sacks_taken, ps.sacks_made, ps.sack_assists_made,
		       ps.field_goal_attempts, ps.field_goal_makes,
		       ps.extra_point_attempts, ps.extra_point_makes,
		       ps.nullified
		FROM nfl_play_statistics ps
		JOIN nfl_plays p ON ps.play_id = p.id
		JOIN nfl_drives d ON p.drive_id = d.id
		WHERE ps.play_id = $1
		  AND p.vendor_deleted = FALSE AND d.vendor_deleted = FALSE
	`

	rows, err := s.Pool().Query(ctx, query, playID)
	if err != nil {
		return nil, fmt.Errorf("failed to query statistics for play_id %d: %w", playID, err)
	}
	defer rows.Close()

	var stats []*models_nfl.PlayStatistic
	for rows.Next() {
		var stat models_nfl.PlayStatistic
		err := rows.Scan(
			&stat.ID,
			&stat.PlayID,
			&stat.IndividualID,
			&stat.StatType,
			&stat.PassingAttempts,
			&stat.RushingAttempts,
			&stat.ReceivingTargets,
			&stat.PassingYards,
			&stat.RushingYards,
			&stat.ReceivingYards,
			&stat.PassingTouchdowns,
			&stat.RushingTouchdowns,
			&stat.ReceivingTouchdowns,
			&stat.PassingCompletions,
			&stat.ReceivingReceptions,
			&stat.InterceptionsThrown,
			&stat.InterceptionsCaught,
			&stat.FumblesCommitted,
			&stat.SacksTaken,
			&stat.SacksMade,
			&stat.SackAssistsMade,
			&stat.FieldGoalAttempts,
			&stat.FieldGoalMakes,
			&stat.ExtraPointAttempts,
			&stat.ExtraPointMakes,
			&stat.Nullified,
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

// GetNFLPlayStatisticsByGameID retrieves all statistics for a given game.
// Joins through nfl_plays -> nfl_drives to filter by game_id.
// Only returns statistics where the associated play and drive are not deleted.
// Returns all PlayStatistic records associated with plays in the specified game.
func GetNFLPlayStatisticsByGameID(s *store.Store, ctx context.Context, gameID string) ([]*models_nfl.PlayStatistic, error) {
	query := `
		SELECT ps.id, ps.play_id, ps.individual_id, ps.stat_type,
		       ps.passing_attempts, ps.rushing_attempts, ps.receiving_targets,
		       ps.passing_yards, ps.rushing_yards, ps.receiving_yards,
		       ps.passing_touchdowns, ps.rushing_touchdowns, ps.receiving_touchdowns,
		       ps.passing_completions, ps.receiving_receptions,
		       ps.interceptions_thrown, ps.interceptions_caught,
		       ps.fumbles_committed,
		       ps.sacks_taken, ps.sacks_made, ps.sack_assists_made,
		       ps.field_goal_attempts, ps.field_goal_makes,
		       ps.extra_point_attempts, ps.extra_point_makes,
		       ps.nullified
		FROM nfl_play_statistics ps
		JOIN nfl_plays p ON ps.play_id = p.id
		JOIN nfl_drives d ON p.drive_id = d.id
		WHERE d.game_id = $1
		  AND p.vendor_deleted = FALSE AND d.vendor_deleted = FALSE
	`

	rows, err := s.Pool().Query(ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query statistics for game_id %s: %w", gameID, err)
	}
	defer rows.Close()

	var stats []*models_nfl.PlayStatistic
	for rows.Next() {
		var stat models_nfl.PlayStatistic
		err := rows.Scan(
			&stat.ID,
			&stat.PlayID,
			&stat.IndividualID,
			&stat.StatType,
			&stat.PassingAttempts,
			&stat.RushingAttempts,
			&stat.ReceivingTargets,
			&stat.PassingYards,
			&stat.RushingYards,
			&stat.ReceivingYards,
			&stat.PassingTouchdowns,
			&stat.RushingTouchdowns,
			&stat.ReceivingTouchdowns,
			&stat.PassingCompletions,
			&stat.ReceivingReceptions,
			&stat.InterceptionsThrown,
			&stat.InterceptionsCaught,
			&stat.FumblesCommitted,
			&stat.SacksTaken,
			&stat.SacksMade,
			&stat.SackAssistsMade,
			&stat.FieldGoalAttempts,
			&stat.FieldGoalMakes,
			&stat.ExtraPointAttempts,
			&stat.ExtraPointMakes,
			&stat.Nullified,
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
