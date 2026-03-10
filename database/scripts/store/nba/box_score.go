package nba

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	models_nba "github.com/openbook/shared/models/nba"
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
	GameID                 string
	IndividualID           string
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
// Sets vendor_deleted = FALSE on both insert and update.
func UpsertNBABoxScore(s *store.Store, ctx context.Context, tx pgx.Tx, boxScore *NBABoxScoreForUpsert) error {
	query := `
		INSERT INTO nba_box_scores (
			game_id, individual_id,
			two_point_attempts, two_point_makes,
			three_point_attempts, three_point_makes,
			free_throw_attempts, free_throw_makes,
			assists, defensive_rebounds, offensive_rebounds,
			steals, blocks, turnovers_committed, personal_fouls_committed,
			vendor_deleted, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			FALSE, NOW(), NOW()
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
			vendor_deleted = FALSE
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
		return fmt.Errorf("failed to upsert box score for game_id %s, individual_id %s: %w",
			boxScore.GameID, boxScore.IndividualID, err)
	}

	return nil
}

// MarkNBABoxScoreDeleted marks an NBA box score as vendor_deleted = TRUE.
// Uses (game_id, individual_id) composite key since box scores no longer have an id column.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func MarkNBABoxScoreDeleted(s *store.Store, ctx context.Context, tx pgx.Tx, gameID string, individualID string) error {
	query := `
		UPDATE nba_box_scores
		SET vendor_deleted = TRUE
		WHERE game_id = $1 AND individual_id = $2
	`
	_, err := tx.Exec(ctx, query, gameID, individualID)
	if err != nil {
		return fmt.Errorf("failed to mark box score as deleted (game_id: %s, individual_id: %s): %w", gameID, individualID, err)
	}

	return nil
}

// GetNBABoxScoresByGameID retrieves all box scores for a game with individual info.
// Uses the registry for caching Game and Individual instances.
// Only returns box scores where vendor_deleted = FALSE.
// Returns a slice of IndividualBoxScore with player info and stats populated.
func GetNBABoxScoresByGameID(s *store.Store, ctx context.Context, gameID string) ([]*models_nba.IndividualBoxScore, error) {
	// Get the game from registry (resolves teams automatically)
	game, err := s.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game %s: %w", gameID, err)
	}

	query := `
		SELECT
			bs.individual_id,
			bs.two_point_attempts, bs.two_point_makes,
			bs.three_point_attempts, bs.three_point_makes,
			bs.free_throw_attempts, bs.free_throw_makes,
			bs.assists, bs.defensive_rebounds, bs.offensive_rebounds,
			bs.steals, bs.blocks, bs.turnovers_committed, bs.personal_fouls_committed
		FROM nba_box_scores bs
		WHERE bs.game_id = $1 AND bs.vendor_deleted = FALSE
		ORDER BY bs.individual_id
	`

	rows, err := s.Pool().Query(ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query box scores for game_id %s: %w", gameID, err)
	}
	defer rows.Close()

	var results []*models_nba.IndividualBoxScore
	for rows.Next() {
		stats := &models_nba.NBAStats{}
		var individualID string

		err := rows.Scan(
			&individualID,
			&stats.TwoPointAttempts,
			&stats.TwoPointMakes,
			&stats.ThreePointAttempts,
			&stats.ThreePointMakes,
			&stats.FreeThrowAttempts,
			&stats.FreeThrowMakes,
			&stats.Assists,
			&stats.DefensiveRebounds,
			&stats.OffensiveRebounds,
			&stats.Steals,
			&stats.Blocks,
			&stats.TurnoversCommitted,
			&stats.PersonalFoulsCommitted,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan box score row: %w", err)
		}

		// Get individual from registry
		individual, err := s.GetIndividualByID(ctx, individualID)
		if err != nil {
			return nil, fmt.Errorf("failed to get individual %s: %w", individualID, err)
		}

		// Set pointer fields
		stats.Game = game
		stats.Individual = individual

		// Register NBAStats in sport-specific registry
		registeredStats := models_nba.Registry.RegisterNBAStats(stats)

		results = append(results, &models_nba.IndividualBoxScore{
			Individual: individual,
			Stats:      registeredStats,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating box score rows: %w", err)
	}

	return results, nil
}

// GetAllNBAGamesWithBoxScores returns all game IDs that have NBA box score entries
// within the specified date range (inclusive).
// Uses the provided IANA timezone to determine which calendar date a game falls on.
// Filters by scheduled_start_time and only considers non-deleted box scores.
// Returns game IDs ordered by scheduled_start_time ascending (earliest games first).
func GetAllNBAGamesWithBoxScores(s *store.Store, ctx context.Context, startDate, endDate time.Time, timeZone string) ([]string, error) {
	query := `
		SELECT DISTINCT bs.game_id, g.scheduled_start_time
		FROM nba_box_scores bs
		JOIN games g ON bs.game_id = g.id
		WHERE (g.scheduled_start_time AT TIME ZONE $3)::date >= $1
		  AND (g.scheduled_start_time AT TIME ZONE $3)::date <= $2
		  AND bs.vendor_deleted = FALSE
		ORDER BY g.scheduled_start_time
	`

	rows, err := s.Pool().Query(ctx, query, startDate, endDate, timeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to query games with NBA box scores: %w", err)
	}
	defer rows.Close()

	var gameIDs []string
	for rows.Next() {
		var gameID string
		var scheduledStartTime time.Time
		if err := rows.Scan(&gameID, &scheduledStartTime); err != nil {
			return nil, fmt.Errorf("failed to scan game_id: %w", err)
		}
		gameIDs = append(gameIDs, gameID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating game IDs: %w", err)
	}

	return gameIDs, nil
}
