package nba

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NBA Play-by-Play Persister
// =============================================================================
// This package handles the persistence of NBA play-by-play data in a single
// atomic transaction. All foreign key lookups are done via database subqueries.
//
// Design principles:
// - Single transaction: All operations succeed together or fail together
// - Database validation: Enums and foreign keys validated by database constraints
// - Subquery lookups: Foreign keys resolved inline via vendor_id lookups
// - Fault-intolerant: Any constraint violation causes full transaction rollback
//
// Key difference from NFL: NBA has no drives - plays are directly under periods.
// =============================================================================

// PersistNBAPlayByPlay persists play-by-play data to the database in a single transaction.
// Only events that pass shouldPersistPlay() are persisted.
//
// The gameID parameter is the database game ID (not vendor UUID).
// All foreign key lookups (players) are done via database subqueries.
// All enum validations are done by the database.
// If any operation fails, the entire transaction is rolled back.
func PersistNBAPlayByPlay(ctx context.Context, dbStore *store.Store, gameID int, pbp *fetcher_nba.PlayByPlayResponse) error {
	// Step 1: Start transaction for all write operations
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

	// Step 2: Upsert game status
	// Map the API status to database enum value
	mappedGameStatus, err := MapGameStatusToDB(pbp.Status)
	if err != nil {
		return fmt.Errorf("failed to map game status: %w", err)
	}
	gameStatus := &store.GameStatusForUpsert{
		GameID: gameID,
		Status: mappedGameStatus,
	}
	if err := dbStore.UpsertGameStatus(ctx, tx, gameStatus); err != nil {
		return fmt.Errorf("failed to upsert game status: %w", err)
	}

	// Step 3: Process all periods and events (plays)
	// NBA has no drives - plays are directly under periods
	for _, period := range pbp.Periods {
		for _, event := range period.Events {
			if err := persistPlay(ctx, dbStore, tx, gameID, &period, &event); err != nil {
				return fmt.Errorf("failed to persist play (vendor_id: %s): %w", event.ID, err)
			}
		}
	}

	// Step 4: Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Mark tx as nil so deferred rollback is a no-op
	tx = nil

	return nil
}

// persistPlay upserts a play and all its statistics within the transaction.
// Foreign keys are resolved via subqueries in the store layer.
func persistPlay(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID int, period *fetcher_nba.Period, event *fetcher_nba.Event) error {
	if !shouldPersistPlay(event) {
		return nil
	}

	// Parse timestamps
	createdAt, err := parseTimestamp(event.Created)
	if err != nil {
		return fmt.Errorf("failed to parse created timestamp: %w", err)
	}
	updatedAt, err := parseTimestamp(event.Updated)
	if err != nil {
		return fmt.Errorf("failed to parse updated timestamp: %w", err)
	}

	// Map API period type to database enum value
	periodType, err := MapPeriodTypeToDB(period.Type)
	if err != nil {
		return fmt.Errorf("failed to map period type: %w", err)
	}

	// Upsert the play
	play := &store_nba.NBAPlayForUpsert{
		GameID:          gameID,
		VendorID:        event.ID,
		VendorSequence:  decimal.NewFromInt(event.Sequence),
		PeriodType:      periodType,
		PeriodNumber:    period.Number,
		Description:     event.Description,
		VendorCreatedAt: createdAt,
		VendorUpdatedAt: updatedAt,
	}

	playID, err := store_nba.UpsertNBAPlay(dbStore, ctx, tx, play)
	if err != nil {
		return err
	}

	// Prepare statistics for upsert
	var stats []*store_nba.NBAPlayStatisticForUpsert
	for _, stat := range event.Statistics {
		if !shouldPersistPlayStatistic(&stat) {
			continue
		}

		// Map API stat type to database enum value
		statType, err := MapStatTypeToDB(stat.Type)
		if err != nil {
			return fmt.Errorf("failed to map stat type: %w", err)
		}

		// Create the base statistic struct with all fields initialized to 0
		playStatistic := &store_nba.NBAPlayStatisticForUpsert{
			VendorPlayerID:         stat.Player.ID,
			StatType:               statType,
			TwoPointAttempts:       decimal.NewFromFloat(0),
			TwoPointMakes:          decimal.NewFromFloat(0),
			ThreePointAttempts:     decimal.NewFromFloat(0),
			ThreePointMakes:        decimal.NewFromFloat(0),
			FreeThrowAttempts:      decimal.NewFromFloat(0),
			FreeThrowMakes:         decimal.NewFromFloat(0),
			Assists:                decimal.NewFromFloat(0),
			DefensiveRebounds:      decimal.NewFromFloat(0),
			OffensiveRebounds:      decimal.NewFromFloat(0),
			Steals:                 decimal.NewFromFloat(0),
			Blocks:                 decimal.NewFromFloat(0),
			TurnoversCommitted:     decimal.NewFromFloat(0),
			PersonalFoulsCommitted: decimal.NewFromFloat(0),
		}

		// Populate stat-type-specific fields based on the stat type
		switch statType {
		case "field_goal":
			if stat.ThreePointShot {
				playStatistic.ThreePointAttempts = decimal.NewFromFloat(1)
				if stat.Made {
					playStatistic.ThreePointMakes = decimal.NewFromFloat(1)
				}
			} else {
				playStatistic.TwoPointAttempts = decimal.NewFromFloat(1)
				if stat.Made {
					playStatistic.TwoPointMakes = decimal.NewFromFloat(1)
				}
			}
		case "free_throw":
			playStatistic.FreeThrowAttempts = decimal.NewFromFloat(1)
			if stat.Made {
				playStatistic.FreeThrowMakes = decimal.NewFromFloat(1)
			}
		case "assist":
			playStatistic.Assists = decimal.NewFromFloat(1)
		case "rebound":
			if stat.ReboundType == "offensive" {
				playStatistic.OffensiveRebounds = decimal.NewFromFloat(1)
			} else {
				// Default to defensive rebound
				playStatistic.DefensiveRebounds = decimal.NewFromFloat(1)
			}
		case "steal":
			playStatistic.Steals = decimal.NewFromFloat(1)
		case "block":
			playStatistic.Blocks = decimal.NewFromFloat(1)
		case "turnover":
			playStatistic.TurnoversCommitted = decimal.NewFromFloat(1)
		case "personal_foul":
			playStatistic.PersonalFoulsCommitted = decimal.NewFromFloat(1)
		default:
			return fmt.Errorf("unexpected stat type after mapping: %s", statType)
		}

		if !shouldPersistPlayStatisticForUpsert(playStatistic) {
			continue
		}

		stats = append(stats, playStatistic)
	}

	// Replace all statistics for this play - player lookups done via subqueries
	if err := store_nba.ReplaceNBAPlayStatistics(dbStore, ctx, tx, playID, stats); err != nil {
		return fmt.Errorf("failed to replace statistics: %w", err)
	}

	return nil
}

// parseTimestamp parses a Sportradar timestamp string (ISO 8601 format)
func parseTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	// Sportradar uses ISO 8601 format: "2025-04-20T00:51:24Z"
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format %q: %w", ts, err)
	}
	return t, nil
}
