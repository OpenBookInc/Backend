package nba

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	reader_nba "github.com/openbook/population-scripts/reader/nba"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	models_nba "github.com/openbook/shared/models/nba"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Box Score Persister
// =============================================================================
// This package handles aggregation of NBA play-by-play statistics into
// box scores and persists them to the database in a single atomic transaction.
//
// Design principles:
// - Single transaction: All box scores for a game succeed together or fail together
// - Aggregation: Sums all stat columns across all plays for each player
// - Fault-intolerant: Any error causes full transaction rollback
//
// Unlike NFL, NBA play statistics do not have a nullified column, so all
// statistics are included in the aggregation.
// =============================================================================

// playerStatsAccumulator holds running totals for a player's statistics
type playerStatsAccumulator struct {
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

// newPlayerStatsAccumulator creates a new accumulator initialized to zero values
func newPlayerStatsAccumulator(individualID int) *playerStatsAccumulator {
	zero := decimal.NewFromInt(0)
	return &playerStatsAccumulator{
		IndividualID:           individualID,
		TwoPointAttempts:       zero,
		TwoPointMakes:          zero,
		ThreePointAttempts:     zero,
		ThreePointMakes:        zero,
		FreeThrowAttempts:      zero,
		FreeThrowMakes:         zero,
		Assists:                zero,
		DefensiveRebounds:      zero,
		OffensiveRebounds:      zero,
		Steals:                 zero,
		Blocks:                 zero,
		TurnoversCommitted:     zero,
		PersonalFoulsCommitted: zero,
	}
}

// PersistNBABoxScores aggregates play-by-play statistics into box scores
// and persists them to the database within the provided transaction.
//
// Aggregation rules:
// - Groups all statistics by individual_id
// - Sums each stat column across all plays for that player
//
// Returns the list of box scores that were upserted, for use with CheckAndUpdateNBABoxScoreDeletions.
//
// IMPORTANT: The caller is responsible for:
// 1. Beginning the transaction and passing it to this function
// 2. Calling CheckAndUpdateNBABoxScoreDeletions() after this function
// 3. Committing the transaction
func PersistNBABoxScores(ctx context.Context, dbStore *store.Store, tx pgx.Tx, data *reader_nba.NBAPlayByPlayData) ([]*store_nba.NBABoxScoreForUpsert, error) {
	// Step 1: Aggregate statistics by player
	accumulators := make(map[int]*playerStatsAccumulator)

	for _, stat := range data.Statistics {
		// Get or create accumulator for this player
		acc, exists := accumulators[stat.IndividualID]
		if !exists {
			acc = newPlayerStatsAccumulator(stat.IndividualID)
			accumulators[stat.IndividualID] = acc
		}

		// Add statistics to accumulator
		acc.TwoPointAttempts = acc.TwoPointAttempts.Add(stat.TwoPointAttempts)
		acc.TwoPointMakes = acc.TwoPointMakes.Add(stat.TwoPointMakes)
		acc.ThreePointAttempts = acc.ThreePointAttempts.Add(stat.ThreePointAttempts)
		acc.ThreePointMakes = acc.ThreePointMakes.Add(stat.ThreePointMakes)
		acc.FreeThrowAttempts = acc.FreeThrowAttempts.Add(stat.FreeThrowAttempts)
		acc.FreeThrowMakes = acc.FreeThrowMakes.Add(stat.FreeThrowMakes)
		acc.Assists = acc.Assists.Add(stat.Assists)
		acc.DefensiveRebounds = acc.DefensiveRebounds.Add(stat.DefensiveRebounds)
		acc.OffensiveRebounds = acc.OffensiveRebounds.Add(stat.OffensiveRebounds)
		acc.Steals = acc.Steals.Add(stat.Steals)
		acc.Blocks = acc.Blocks.Add(stat.Blocks)
		acc.TurnoversCommitted = acc.TurnoversCommitted.Add(stat.TurnoversCommitted)
		acc.PersonalFoulsCommitted = acc.PersonalFoulsCommitted.Add(stat.PersonalFoulsCommitted)
	}

	// If no statistics to persist, return empty slice
	if len(accumulators) == 0 {
		return []*store_nba.NBABoxScoreForUpsert{}, nil
	}

	// Step 2: Convert accumulators to box scores and upsert
	var upsertedBoxScores []*store_nba.NBABoxScoreForUpsert
	for _, acc := range accumulators {
		boxScore := &store_nba.NBABoxScoreForUpsert{
			GameID:                 data.GameID,
			IndividualID:           acc.IndividualID,
			TwoPointAttempts:       acc.TwoPointAttempts,
			TwoPointMakes:          acc.TwoPointMakes,
			ThreePointAttempts:     acc.ThreePointAttempts,
			ThreePointMakes:        acc.ThreePointMakes,
			FreeThrowAttempts:      acc.FreeThrowAttempts,
			FreeThrowMakes:         acc.FreeThrowMakes,
			Assists:                acc.Assists,
			DefensiveRebounds:      acc.DefensiveRebounds,
			OffensiveRebounds:      acc.OffensiveRebounds,
			Steals:                 acc.Steals,
			Blocks:                 acc.Blocks,
			TurnoversCommitted:     acc.TurnoversCommitted,
			PersonalFoulsCommitted: acc.PersonalFoulsCommitted,
		}

		if err := store_nba.UpsertNBABoxScore(dbStore, ctx, tx, boxScore); err != nil {
			return nil, fmt.Errorf("failed to upsert box score for individual_id %d: %w", acc.IndividualID, err)
		}
		upsertedBoxScores = append(upsertedBoxScores, boxScore)
	}

	return upsertedBoxScores, nil
}

// GetBoxScoreCount returns the number of box scores that would be generated
// This is useful for printing summaries
func GetBoxScoreCount(data *reader_nba.NBAPlayByPlayData) int {
	players := make(map[int]bool)
	for _, stat := range data.Statistics {
		players[stat.IndividualID] = true
	}
	return len(players)
}

// CheckAndUpdateNBABoxScoreDeletions marks box scores as deleted if they exist
// in the database but were not upserted in the current persist operation.
//
// Parameters:
// - existingBoxScores: The box scores that existed in the database before the persist operation
// - upsertedBoxScores: The box scores that were upserted in the persist operation
//
// For any box score in existingBoxScores that doesn't have a matching individual_id
// in upsertedBoxScores, this function marks it as deleted.
//
// The caller is responsible for committing the transaction after this function returns.
func CheckAndUpdateNBABoxScoreDeletions(ctx context.Context, dbStore *store.Store, tx pgx.Tx, existingBoxScores []*models_nba.IndividualBoxScore, upsertedBoxScores []*store_nba.NBABoxScoreForUpsert) error {
	// Build a set of individual IDs that were upserted
	upsertedIndividuals := make(map[int]bool)
	for _, boxScore := range upsertedBoxScores {
		upsertedIndividuals[boxScore.IndividualID] = true
	}

	// Check each existing box score
	for _, existing := range existingBoxScores {
		if !upsertedIndividuals[existing.Individual.ID] {
			// This box score exists in the database but was not upserted - mark it as deleted
			if err := store_nba.MarkNBABoxScoreDeleted(dbStore, ctx, tx, existing.Stats.ID); err != nil {
				return fmt.Errorf("failed to mark box score as deleted (id: %d, individual_id: %d): %w", existing.Stats.ID, existing.Individual.ID, err)
			}
		}
	}

	return nil
}
