package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/shared/models"
	nflmodels "github.com/openbook/shared/models/nfl"
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

// GetNFLBoxScoresByGameID retrieves all box scores for a game with individual info.
// Uses JOIN to fetch individual details along with stats.
// Also joins with rosters to determine team membership.
// Returns a slice of IndividualBoxScore with player info, stats, and team_id populated.
func (s *Store) GetNFLBoxScoresByGameID(ctx context.Context, gameID int) ([]*nflmodels.IndividualBoxScore, error) {
	query := `
		SELECT
			bs.id, bs.game_id, bs.individual_id,
			bs.completions, bs.incompletions, bs.receptions,
			bs.interceptions, bs.fumbles, bs.fumbles_lost,
			bs.sacks, bs.tackles, bs.assists,
			bs.passing_attempts, bs.rushing_attempts, bs.receiving_targets,
			bs.passing_yards, bs.rushing_yards, bs.receiving_yards,
			bs.passing_touchdowns, bs.rushing_touchdowns, bs.receiving_touchdowns,
			bs.interceptions_thrown, bs.sacks_taken,
			i.id, i.vendor_id, i.display_name, i.abbreviated_name,
			i.date_of_birth, i.league_id, i.position, i.jersey_number,
			COALESCE(r.team_id, 0)
		FROM nfl_box_scores bs
		JOIN individuals i ON bs.individual_id = i.id
		LEFT JOIN rosters r ON i.id = ANY(r.individual_ids)
		WHERE bs.game_id = $1
		ORDER BY i.display_name
	`

	rows, err := s.pool.Query(ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query box scores for game_id %d: %w", gameID, err)
	}
	defer rows.Close()

	var results []*nflmodels.IndividualBoxScore
	for rows.Next() {
		stats := &nflmodels.NFLStats{}
		individual := &models.Individual{}
		var teamID int

		err := rows.Scan(
			&stats.ID,
			&stats.GameID,
			&stats.IndividualID,
			&stats.Completions,
			&stats.Incompletions,
			&stats.Receptions,
			&stats.Interceptions,
			&stats.Fumbles,
			&stats.FumblesLost,
			&stats.Sacks,
			&stats.Tackles,
			&stats.Assists,
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
			&individual.ID,
			&individual.VendorID,
			&individual.DisplayName,
			&individual.AbbreviatedName,
			&individual.DateOfBirth,
			&individual.LeagueID,
			&individual.Position,
			&individual.JerseyNumber,
			&teamID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan box score row: %w", err)
		}

		results = append(results, &nflmodels.IndividualBoxScore{
			Individual: individual,
			Stats:      stats,
			TeamID:     teamID,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating box score rows: %w", err)
	}

	return results, nil
}
