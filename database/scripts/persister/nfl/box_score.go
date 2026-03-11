package nfl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	reader_nfl "github.com/openbook/population-scripts/reader/nfl"
	"github.com/openbook/population-scripts/store"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/shared/utils"
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
	IndividualID        utils.UUID
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

// newPlayerStatsAccumulator creates a new accumulator initialized to zero values
func newPlayerStatsAccumulator(individualID utils.UUID) *playerStatsAccumulator {
	zero := decimal.NewFromInt(0)
	return &playerStatsAccumulator{
		IndividualID:        individualID,
		PassingCompletions:  zero,
		ReceivingReceptions: zero,
		InterceptionsCaught: zero,
		FumblesCommitted:    zero,
		SacksMade:           zero,
		SackAssistsMade:     zero,
		FieldGoalAttempts:   zero,
		FieldGoalMakes:      zero,
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
// and persists them to the database within the provided transaction.
//
// Aggregation rules:
// - Groups all statistics by individual_id
// - Sums each stat column across all plays for that player
// - Excludes statistics where nullified = true
//
// Returns the list of box scores that were upserted, for use with CheckAndUpdateNFLBoxScoreDeletions.
//
// IMPORTANT: The caller is responsible for:
// 1. Beginning the transaction and passing it to this function
// 2. Calling CheckAndUpdateNFLBoxScoreDeletions() after this function
// 3. Committing the transaction
func PersistNFLBoxScores(ctx context.Context, dbStore *store.Store, tx pgx.Tx, data *reader_nfl.NFLPlayByPlayData) ([]*store_nfl.NFLBoxScoreForUpsert, error) {
	// Step 1: Aggregate statistics by player
	accumulators := make(map[utils.UUID]*playerStatsAccumulator)

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
		acc.FumblesCommitted = acc.FumblesCommitted.Add(stat.FumblesCommitted)
		acc.SacksMade = acc.SacksMade.Add(stat.SacksMade)
		acc.SackAssistsMade = acc.SackAssistsMade.Add(stat.SackAssistsMade)
		acc.FieldGoalAttempts = acc.FieldGoalAttempts.Add(stat.FieldGoalAttempts)
		acc.FieldGoalMakes = acc.FieldGoalMakes.Add(stat.FieldGoalMakes)
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

	// If no statistics to persist, return empty slice
	if len(accumulators) == 0 {
		return []*store_nfl.NFLBoxScoreForUpsert{}, nil
	}

	// Step 2: Convert accumulators to box scores and upsert
	var upsertedBoxScores []*store_nfl.NFLBoxScoreForUpsert
	for _, acc := range accumulators {
		boxScore := &store_nfl.NFLBoxScoreForUpsert{
			GameID:              data.GameID,
			IndividualID:        acc.IndividualID.String(),
			PassingCompletions:  acc.PassingCompletions,
			ReceivingReceptions: acc.ReceivingReceptions,
			InterceptionsCaught: acc.InterceptionsCaught,
			FumblesCommitted:    acc.FumblesCommitted,
			SacksMade:           acc.SacksMade,
			SackAssistsMade:     acc.SackAssistsMade,
			FieldGoalAttempts:   acc.FieldGoalAttempts,
			FieldGoalMakes:      acc.FieldGoalMakes,
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
			return nil, fmt.Errorf("failed to upsert box score for individual_id %s: %w", acc.IndividualID, err)
		}
		upsertedBoxScores = append(upsertedBoxScores, boxScore)
	}

	return upsertedBoxScores, nil
}

// GetBoxScoreCount returns the number of box scores that would be generated
// This is useful for printing summaries
func GetBoxScoreCount(data *reader_nfl.NFLPlayByPlayData) int {
	players := make(map[utils.UUID]bool)
	for _, stat := range data.Statistics {
		if !stat.Nullified {
			players[stat.IndividualID] = true
		}
	}
	return len(players)
}

// CheckAndUpdateNFLBoxScoreDeletions marks box scores as deleted if they exist
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
func CheckAndUpdateNFLBoxScoreDeletions(ctx context.Context, dbStore *store.Store, tx pgx.Tx, existingBoxScores []*models_nfl.IndividualBoxScore, upsertedBoxScores []*store_nfl.NFLBoxScoreForUpsert) error {
	// Build a set of individual IDs that were upserted
	upsertedIndividuals := make(map[string]bool)
	for _, boxScore := range upsertedBoxScores {
		upsertedIndividuals[boxScore.IndividualID] = true
	}

	// Check each existing box score
	for _, existing := range existingBoxScores {
		if !upsertedIndividuals[existing.Individual.ID.String()] {
			// This box score exists in the database but was not upserted - mark it as deleted
			if err := store_nfl.MarkNFLBoxScoreDeleted(dbStore, ctx, tx, existing.Stats.Game.ID.String(), existing.Individual.ID.String()); err != nil {
				return fmt.Errorf("failed to mark box score as deleted (game_id: %s, individual_id: %s): %w", existing.Stats.Game.ID, existing.Individual.ID, err)
			}
		}
	}

	return nil
}
