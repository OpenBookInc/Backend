package nfl

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
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
	PassingCompletions  decimal.Decimal
	ReceivingReceptions decimal.Decimal
	InterceptionsCaught decimal.Decimal
	FumblesCommitted    decimal.Decimal
	SacksMade           decimal.Decimal
	SackAssistsMade     decimal.Decimal
	FieldGoalAttempts   decimal.Decimal
	FieldGoalMakes      decimal.Decimal
	ExtraPointAttempts  decimal.Decimal
	ExtraPointMakes     decimal.Decimal
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
func UpsertNFLBoxScore(s *store.Store, ctx context.Context, tx pgx.Tx, boxScore *NFLBoxScoreForUpsert) error {
	query := `
		INSERT INTO nfl_box_scores (
			game_id, individual_id,
			passing_completions, receiving_receptions,
			interceptions_caught, fumbles_committed,
			sacks_made, sack_assists_made,
			field_goal_attempts, field_goal_makes,
			extra_point_attempts, extra_point_makes,
			passing_attempts, rushing_attempts, receiving_targets,
			passing_yards, rushing_yards, receiving_yards,
			passing_touchdowns, rushing_touchdowns, receiving_touchdowns,
			interceptions_thrown, sacks_taken,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23,
			NOW(), NOW()
		)
		ON CONFLICT (game_id, individual_id)
		DO UPDATE SET
			passing_completions = EXCLUDED.passing_completions,
			receiving_receptions = EXCLUDED.receiving_receptions,
			interceptions_caught = EXCLUDED.interceptions_caught,
			fumbles_committed = EXCLUDED.fumbles_committed,
			sacks_made = EXCLUDED.sacks_made,
			sack_assists_made = EXCLUDED.sack_assists_made,
			field_goal_attempts = EXCLUDED.field_goal_attempts,
			field_goal_makes = EXCLUDED.field_goal_makes,
			extra_point_attempts = EXCLUDED.extra_point_attempts,
			extra_point_makes = EXCLUDED.extra_point_makes,
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
		boxScore.PassingCompletions,
		boxScore.ReceivingReceptions,
		boxScore.InterceptionsCaught,
		boxScore.FumblesCommitted,
		boxScore.SacksMade,
		boxScore.SackAssistsMade,
		boxScore.FieldGoalAttempts,
		boxScore.FieldGoalMakes,
		boxScore.ExtraPointAttempts,
		boxScore.ExtraPointMakes,
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

// GetNFLBoxScoresByGameID retrieves all box scores for a game with individual info.
// Uses the registry for caching Game and Individual instances.
// Returns a slice of IndividualBoxScore with player info and stats populated.
func GetNFLBoxScoresByGameID(s *store.Store, ctx context.Context, gameID int) ([]*models_nfl.IndividualBoxScore, error) {
	// Get the game from registry (resolves teams automatically)
	game, err := s.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game %d: %w", gameID, err)
	}

	query := `
		SELECT
			bs.id, bs.individual_id,
			bs.passing_completions, bs.receiving_receptions,
			bs.interceptions_caught, bs.fumbles_committed,
			bs.sacks_made, bs.sack_assists_made,
			bs.field_goal_attempts, bs.field_goal_makes,
			bs.extra_point_attempts, bs.extra_point_makes,
			bs.passing_attempts, bs.rushing_attempts, bs.receiving_targets,
			bs.passing_yards, bs.rushing_yards, bs.receiving_yards,
			bs.passing_touchdowns, bs.rushing_touchdowns, bs.receiving_touchdowns,
			bs.interceptions_thrown, bs.sacks_taken
		FROM nfl_box_scores bs
		WHERE bs.game_id = $1
		ORDER BY bs.individual_id
	`

	rows, err := s.Pool().Query(ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query box scores for game_id %d: %w", gameID, err)
	}
	defer rows.Close()

	var results []*models_nfl.IndividualBoxScore
	for rows.Next() {
		stats := &models_nfl.NFLStats{}
		var individualID int

		err := rows.Scan(
			&stats.ID,
			&individualID,
			&stats.PassingCompletions,
			&stats.ReceivingReceptions,
			&stats.InterceptionsCaught,
			&stats.FumblesCommitted,
			&stats.SacksMade,
			&stats.SackAssistsMade,
			&stats.FieldGoalAttempts,
			&stats.FieldGoalMakes,
			&stats.ExtraPointAttempts,
			&stats.ExtraPointMakes,
			&stats.PassingAttempts,
			&stats.RushingAttempts,
			&stats.ReceivingTargets,
			&stats.PassingYards,
			&stats.RushingYards,
			&stats.ReceivingYards,
			&stats.PassingTouchdowns,
			&stats.RushingTouchdowns,
			&stats.ReceivingTouchdowns,
			&stats.InterceptionsThrown,
			&stats.SacksTaken,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan box score row: %w", err)
		}

		// Get individual from registry
		individual, err := s.GetIndividualByID(ctx, individualID)
		if err != nil {
			return nil, fmt.Errorf("failed to get individual %d: %w", individualID, err)
		}

		// Set pointer fields
		stats.Game = game
		stats.Individual = individual

		// Register NFLStats in sport-specific registry
		registeredStats := models_nfl.Registry.RegisterNFLStats(stats)

		results = append(results, &models_nfl.IndividualBoxScore{
			Individual: individual,
			Stats:      registeredStats,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating box score rows: %w", err)
	}

	return results, nil
}

// GetAllNFLGamesWithBoxScores returns all game IDs that have NFL box score entries
// within the specified date range (inclusive).
// Filters by scheduled_start_time.
// Returns game IDs ordered by game_id ascending.
func GetAllNFLGamesWithBoxScores(s *store.Store, ctx context.Context, startDate, endDate time.Time) ([]int, error) {
	query := `
		SELECT DISTINCT bs.game_id
		FROM nfl_box_scores bs
		JOIN games g ON bs.game_id = g.id
		WHERE g.scheduled_start_time >= $1 AND DATE(g.scheduled_start_time) <= DATE($2)
		ORDER BY bs.game_id
	`

	rows, err := s.Pool().Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query games with NFL box scores: %w", err)
	}
	defer rows.Close()

	var gameIDs []int
	for rows.Next() {
		var gameID int
		if err := rows.Scan(&gameID); err != nil {
			return nil, fmt.Errorf("failed to scan game_id: %w", err)
		}
		gameIDs = append(gameIDs, gameID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game IDs: %w", err)
	}

	return gameIDs, nil
}
