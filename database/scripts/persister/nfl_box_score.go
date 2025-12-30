package persister

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/reader"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Box Score Persister
// =============================================================================
// This package handles aggregation of NFL play-by-play statistics into
// box scores and persists them to the database in a single atomic transaction.
//
// Design principles:
// - Single transaction: All box scores for a game succeed together or fail together
// - Aggregation: Sums all stat columns across all plays for each player
// - Nullified exclusion: Statistics with nullified=true are excluded from sums
// - Fault-intolerant: Any error causes full transaction rollback
// =============================================================================

// playerStatsAccumulator holds running totals for a player's statistics
type playerStatsAccumulator struct {
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

// newPlayerStatsAccumulator creates a new accumulator initialized to zero values
func newPlayerStatsAccumulator(individualID int) *playerStatsAccumulator {
	zero := decimal.NewFromInt(0)
	return &playerStatsAccumulator{
		IndividualID:        individualID,
		Completions:         zero,
		Incompletions:       zero,
		Receptions:          zero,
		Interceptions:       zero,
		Fumbles:             zero,
		FumblesLost:         zero,
		Sacks:               zero,
		Tackles:             zero,
		Assists:             zero,
		PassingAttempts:     zero,
		RushingAttempts:     zero,
		ReceivingTargets:    zero,
		PassingYards:        zero,
		RushingYards:        zero,
		ReceivingYards:      zero,
		PassingTouchdowns:   zero,
		RushingTouchdowns:   zero,
		ReceivingTouchdowns: zero,
		InterceptionsThrown: zero,
		SacksTaken:          zero,
	}
}

// PersistNFLBoxScores aggregates play-by-play statistics into box scores
// and persists them to the database in a single transaction.
//
// Aggregation rules:
// - Groups all statistics by individual_id
// - Sums each stat column across all plays for that player
// - Excludes statistics where nullified = true
//
// All box scores for the game are upserted atomically - if any fails,
// the entire transaction is rolled back.
func PersistNFLBoxScores(ctx context.Context, dbStore *store.Store, data *reader.NFLPlayByPlayData) error {
	// Step 1: Aggregate statistics by player
	accumulators := make(map[int]*playerStatsAccumulator)

	for _, stat := range data.Statistics {
		// Skip nullified statistics
		if stat.Nullified {
			continue
		}

		// Get or create accumulator for this player
		acc, exists := accumulators[stat.IndividualID]
		if !exists {
			acc = newPlayerStatsAccumulator(stat.IndividualID)
			accumulators[stat.IndividualID] = acc
		}

		// Add statistics to accumulator
		acc.Completions = acc.Completions.Add(stat.Completions)
		acc.Incompletions = acc.Incompletions.Add(stat.Incompletions)
		acc.Receptions = acc.Receptions.Add(stat.Receptions)
		acc.Interceptions = acc.Interceptions.Add(stat.Interceptions)
		acc.Fumbles = acc.Fumbles.Add(stat.Fumbles)
		acc.FumblesLost = acc.FumblesLost.Add(stat.FumblesLost)
		acc.Sacks = acc.Sacks.Add(stat.Sacks)
		acc.Tackles = acc.Tackles.Add(stat.Tackles)
		acc.Assists = acc.Assists.Add(stat.Assists)
		acc.PassingAttempts = acc.PassingAttempts.Add(stat.PassingAttempts)
		acc.RushingAttempts = acc.RushingAttempts.Add(stat.RushingAttempts)
		acc.ReceivingTargets = acc.ReceivingTargets.Add(stat.ReceivingTargets)
		acc.PassingYards = acc.PassingYards.Add(stat.PassingYards)
		acc.RushingYards = acc.RushingYards.Add(stat.RushingYards)
		acc.ReceivingYards = acc.ReceivingYards.Add(stat.ReceivingYards)
		acc.PassingTouchdowns = acc.PassingTouchdowns.Add(stat.PassingTouchdowns)
		acc.RushingTouchdowns = acc.RushingTouchdowns.Add(stat.RushingTouchdowns)
		acc.ReceivingTouchdowns = acc.ReceivingTouchdowns.Add(stat.ReceivingTouchdowns)
		acc.InterceptionsThrown = acc.InterceptionsThrown.Add(stat.InterceptionsThrown)
		acc.SacksTaken = acc.SacksTaken.Add(stat.SacksTaken)
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
		boxScore := &store.NFLBoxScoreForUpsert{
			GameID:              data.GameID,
			IndividualID:        acc.IndividualID,
			Completions:         acc.Completions,
			Incompletions:       acc.Incompletions,
			Receptions:          acc.Receptions,
			Interceptions:       acc.Interceptions,
			Fumbles:             acc.Fumbles,
			FumblesLost:         acc.FumblesLost,
			Sacks:               acc.Sacks,
			Tackles:             acc.Tackles,
			Assists:             acc.Assists,
			PassingAttempts:     acc.PassingAttempts,
			RushingAttempts:     acc.RushingAttempts,
			ReceivingTargets:    acc.ReceivingTargets,
			PassingYards:        acc.PassingYards,
			RushingYards:        acc.RushingYards,
			ReceivingYards:      acc.ReceivingYards,
			PassingTouchdowns:   acc.PassingTouchdowns,
			RushingTouchdowns:   acc.RushingTouchdowns,
			ReceivingTouchdowns: acc.ReceivingTouchdowns,
			InterceptionsThrown: acc.InterceptionsThrown,
			SacksTaken:          acc.SacksTaken,
		}

		if err := dbStore.UpsertNFLBoxScore(ctx, tx, boxScore); err != nil {
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
func GetBoxScoreCount(data *reader.NFLPlayByPlayData) int {
	players := make(map[int]bool)
	for _, stat := range data.Statistics {
		if !stat.Nullified {
			players[stat.IndividualID] = true
		}
	}
	return len(players)
}
