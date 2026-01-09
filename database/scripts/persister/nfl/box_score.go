package nfl

import (
	"context"
	"fmt"

	reader_nfl "github.com/openbook/population-scripts/reader/nfl"
	"github.com/openbook/population-scripts/store"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
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
	PassingCompletions  decimal.Decimal
	ReceivingReceptions decimal.Decimal
	InterceptionsCaught decimal.Decimal
	FumblesForced       decimal.Decimal
	FumblesCommitted         decimal.Decimal
	SacksMade           decimal.Decimal
	SackAssistsMade     decimal.Decimal
	TacklesMade         decimal.Decimal
	TackleAssistsMade   decimal.Decimal
	FieldGoalAttempts   decimal.Decimal
	FieldGoalMakes      decimal.Decimal
	FieldGoalMakeYards  decimal.Decimal
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

// newPlayerStatsAccumulator creates a new accumulator initialized to zero values
func newPlayerStatsAccumulator(individualID int) *playerStatsAccumulator {
	zero := decimal.NewFromInt(0)
	return &playerStatsAccumulator{
		IndividualID:        individualID,
		PassingCompletions:  zero,
		ReceivingReceptions: zero,
		InterceptionsCaught: zero,
		FumblesForced:       zero,
		FumblesCommitted:         zero,
		SacksMade:           zero,
		SackAssistsMade:     zero,
		TacklesMade:         zero,
		TackleAssistsMade:   zero,
		FieldGoalAttempts:   zero,
		FieldGoalMakes:      zero,
		FieldGoalMakeYards:  zero,
		ExtraPointAttempts:  zero,
		ExtraPointMakes:     zero,
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
func PersistNFLBoxScores(ctx context.Context, dbStore *store.Store, data *reader_nfl.NFLPlayByPlayData) error {
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
		acc.PassingCompletions = acc.PassingCompletions.Add(stat.PassingCompletions)
		acc.ReceivingReceptions = acc.ReceivingReceptions.Add(stat.ReceivingReceptions)
		acc.InterceptionsCaught = acc.InterceptionsCaught.Add(stat.InterceptionsCaught)
		acc.FumblesForced = acc.FumblesForced.Add(stat.FumblesForced)
		acc.FumblesCommitted = acc.FumblesCommitted.Add(stat.FumblesCommitted)
		acc.SacksMade = acc.SacksMade.Add(stat.SacksMade)
		acc.SackAssistsMade = acc.SackAssistsMade.Add(stat.SackAssistsMade)
		acc.TacklesMade = acc.TacklesMade.Add(stat.TacklesMade)
		acc.TackleAssistsMade = acc.TackleAssistsMade.Add(stat.TackleAssistsMade)
		acc.FieldGoalAttempts = acc.FieldGoalAttempts.Add(stat.FieldGoalAttempts)
		acc.FieldGoalMakes = acc.FieldGoalMakes.Add(stat.FieldGoalMakes)
		acc.FieldGoalMakeYards = acc.FieldGoalMakeYards.Add(stat.FieldGoalMakeYards)
		acc.ExtraPointAttempts = acc.ExtraPointAttempts.Add(stat.ExtraPointAttempts)
		acc.ExtraPointMakes = acc.ExtraPointMakes.Add(stat.ExtraPointMakes)
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
		boxScore := &store_nfl.NFLBoxScoreForUpsert{
			GameID:              data.GameID,
			IndividualID:        acc.IndividualID,
			PassingCompletions:  acc.PassingCompletions,
			ReceivingReceptions: acc.ReceivingReceptions,
			InterceptionsCaught: acc.InterceptionsCaught,
			FumblesForced:       acc.FumblesForced,
			FumblesCommitted:         acc.FumblesCommitted,
			SacksMade:           acc.SacksMade,
			SackAssistsMade:     acc.SackAssistsMade,
			TacklesMade:         acc.TacklesMade,
			TackleAssistsMade:   acc.TackleAssistsMade,
			FieldGoalAttempts:   acc.FieldGoalAttempts,
			FieldGoalMakes:      acc.FieldGoalMakes,
			FieldGoalMakeYards:  acc.FieldGoalMakeYards,
			ExtraPointAttempts:  acc.ExtraPointAttempts,
			ExtraPointMakes:     acc.ExtraPointMakes,
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

		if err := store_nfl.UpsertNFLBoxScore(dbStore, ctx, tx, boxScore); err != nil {
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
func GetBoxScoreCount(data *reader_nfl.NFLPlayByPlayData) int {
	players := make(map[int]bool)
	for _, stat := range data.Statistics {
		if !stat.Nullified {
			players[stat.IndividualID] = true
		}
	}
	return len(players)
}
