package nba

import (
	"context"
	"fmt"

	reader_nba "github.com/openbook/population-scripts/reader/nba"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
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
// and persists them to the database in a single transaction.
//
// Aggregation rules:
// - Groups all statistics by individual_id
// - Sums each stat column across all plays for that player
//
// All box scores for the game are upserted atomically - if any fails,
// the entire transaction is rolled back.
func PersistNBABoxScores(ctx context.Context, dbStore *store.Store, data *reader_nba.NBAPlayByPlayData) error {
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

	// If no statistics to persist, return early
	if len(accumulators) == 0 {
		return nil
	}

	// Step 2: Begin transaction
	tx, err := dbStore.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback - no-op if commit succeeds
	defer func() {
		if tx != nil {
			tx.Rollback(ctx)
		}
	}()

	// Step 3: Convert accumulators to box scores and upsert
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
			return fmt.Errorf("failed to upsert box score for individual_id %d: %w", acc.IndividualID, err)
		}
	}

	// Step 4: Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Mark tx as nil so deferred rollback is a no-op
	tx = nil

	return nil
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
