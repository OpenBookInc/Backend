package nba

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/client/sportradar"
	fetcher_nba "github.com/openbook/population-scripts/fetcher/nba"
	"github.com/openbook/population-scripts/persister"
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
// - Subquery lookups: Foreign keys resolved inline via sportradar_id lookups
// - Fault-intolerant: Any constraint violation causes full transaction rollback
//
// Key difference from NFL: NBA has no drives - plays are directly under periods.
// =============================================================================

// PersistMissingNBAIndividuals ensures all players referenced in the play-by-play data
// exist in the database. For any missing players, it fetches their profile from the
// Sportradar API and persists them.
//
// This function should be called BEFORE starting the transaction, as it may make
// API calls and individual database writes that should not be part of the main transaction.
func PersistMissingNBAIndividuals(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, pbp *fetcher_nba.PlayByPlayResponse) error {
	return persistMissingIndividuals(ctx, dbStore, apiClient, pbp)
}

// PersistNBAPlayByPlay persists play-by-play data to the database within the provided transaction.
// Only events that pass shouldPersistPlay() are persisted.
//
// The gameID parameter is the database game ID (not vendor UUID).
// All foreign key lookups (players) are done via database subqueries.
// All enum validations are done by the database.
//
// IMPORTANT: The caller is responsible for:
// 1. Calling PersistMissingNBAIndividuals() before starting the transaction
// 2. Beginning the transaction and passing it to this function
// 3. Calling CheckAndUpdateNBAPlayByPlayDeletions() after this function
// 4. Committing the transaction
func PersistNBAPlayByPlay(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID string, pbp *fetcher_nba.PlayByPlayResponse) error {
	// Step 1: Upsert game status
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

	// Step 2: Process all periods and events (plays)
	// NBA has no drives - plays are directly under periods
	for _, period := range pbp.Periods {
		for _, event := range period.Events {
			if err := persistPlay(ctx, dbStore, tx, gameID, &period, &event); err != nil {
				return fmt.Errorf("failed to persist play (sportradar_id: %s): %w", event.ID, err)
			}
		}
	}

	return nil
}

// persistPlay upserts a play and all its statistics within the transaction.
// Foreign keys are resolved via subqueries in the store layer.
func persistPlay(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID string, period *fetcher_nba.Period, event *fetcher_nba.Event) error {
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
		SportradarID:    event.ID,
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

// persistMissingIndividuals ensures all players referenced in the play-by-play data
// exist in the database. For any missing players, it fetches their profile from the
// Sportradar API and persists them.
func persistMissingIndividuals(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, pbp *fetcher_nba.PlayByPlayResponse) error {
	// Extract all unique player sportradar IDs from persistable statistics
	playerSportradarIDs := ExtractPlayerSportradarIDs(pbp)

	// Check each player and fetch/persist if missing
	for _, sportradarID := range playerSportradarIDs {
		individual, created, err := persister.UpsertIndividualIfMissing(ctx, dbStore, apiClient, sportradarID, "NBA")
		if err != nil {
			return fmt.Errorf("failed to ensure player exists (sportradar_id: %s): %w", sportradarID, err)
		}
		if created {
			fmt.Printf("  Persisted missing player: %s\n", individual.DisplayName)
		}
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

// CheckAndUpdateNBAPlayByPlayDeletions marks plays as deleted in the database
// if they are marked as deleted in the fetcher response.
//
// This function iterates through all plays in the fetcher response and marks
// any with Deleted=true as deleted in the database.
//
// The caller is responsible for committing the transaction after this function returns.
func CheckAndUpdateNBAPlayByPlayDeletions(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID string, pbp *fetcher_nba.PlayByPlayResponse) error {
	for _, period := range pbp.Periods {
		for _, event := range period.Events {
			if event.Deleted {
				// Look up the play in the database (only finds non-deleted plays)
				existingPlay, err := store_nba.GetNBAPlayBySportradarID(dbStore, ctx, gameID, event.ID)
				if err != nil {
					// Play doesn't exist or is already deleted - nothing to do
					continue
				}
				// Mark the play as deleted
				if err := store_nba.MarkNBAPlayDeleted(dbStore, ctx, tx, existingPlay.ID); err != nil {
					return fmt.Errorf("failed to mark play as deleted (sportradar_id: %s): %w", event.ID, err)
				}
			}
		}
	}

	return nil
}
