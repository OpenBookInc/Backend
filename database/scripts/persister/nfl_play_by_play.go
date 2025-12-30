package persister

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// =============================================================================
// NFL Play-by-Play Persister
// =============================================================================
// This package handles the persistence of NFL play-by-play data in a single
// atomic transaction. All foreign key lookups are done via database subqueries.
//
// Design principles:
// - Single transaction: All operations succeed together or fail together
// - Database validation: Enums and foreign keys validated by database constraints
// - Subquery lookups: Foreign keys resolved inline via vendor_id lookups
// - Fault-intolerant: Any constraint violation causes full transaction rollback
// =============================================================================

// PersistNFLPlayByPlay persists play-by-play data to the database in a single transaction.
// Only events with Type == "play" AND Official == true are persisted.
// These represent official, confirmed plays (not timeouts, penalties without plays, etc.)
//
// The gameID parameter is the database game ID (not vendor UUID).
// All foreign key lookups (teams, players) are done via database subqueries.
// All enum validations are done by the database.
// If any operation fails, the entire transaction is rolled back.
func PersistNFLPlayByPlay(ctx context.Context, dbStore *store.Store, gameID int, pbp *nfl.PlayByPlayResponse) error {
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

	// Step 3: Process all drives, plays, and statistics
	for _, period := range pbp.Periods {
		for _, drive := range period.PBP {
			if err := persistDrive(ctx, dbStore, tx, gameID, &period, &drive); err != nil {
				return fmt.Errorf("failed to persist drive (vendor_id: %s): %w", drive.ID, err)
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

// persistDrive upserts a drive and all its plays within the transaction.
// Only processes drives with type "drive" (skips "event" entries like timeouts).
// Foreign keys are resolved via subqueries in the store layer.
func persistDrive(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID int, period *nfl.Period, drive *nfl.Drive) error {
	// Skip non-drive entries (e.g., type "event" for timeouts, end-of-period markers)
	if !shouldPersistDrive(drive) {
		return nil
	}

	// Validate that drive has offensive team information.
	if drive.OffensiveTeam == nil {
		return fmt.Errorf("drive missing offensive team information")
	}

	// Upsert the drive - team lookup done via subquery
	driveID, err := dbStore.UpsertNFLDrive(
		ctx,
		tx,
		gameID,
		drive.ID,
		decimal.NewFromFloat(drive.Sequence),
		drive.OffensiveTeam.ID, // This is the vendor_id, not db id
	)
	if err != nil {
		return err
	}

	// Process each event in the drive
	for _, event := range drive.Events {
		// Only persist events that are official, confirmed plays
		// - Type == "play": Filters out timeouts, end-of-period markers, etc.
		// - Official == true: Filters out tentative plays that may be reversed
		if event.Type != "play" || !event.Official {
			continue
		}

		if err := persistPlay(ctx, dbStore, tx, driveID, period, &event); err != nil {
			return fmt.Errorf("failed to persist play (vendor_id: %s): %w", event.ID, err)
		}
	}

	return nil
}

// persistPlay upserts a play and all its statistics within the transaction.
// Foreign keys are resolved via subqueries in the store layer.
func persistPlay(ctx context.Context, dbStore *store.Store, tx pgx.Tx, driveID int, period *nfl.Period, event *nfl.Event) error {
	// Parse timestamps
	createdAt, err := parseTimestamp(event.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to parse created_at timestamp: %w", err)
	}
	updatedAt, err := parseTimestamp(event.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to parse updated_at timestamp: %w", err)
	}

	// Determine if play is nullified (any stat is nullified)
	nullified := false
	for _, stat := range event.Statistics {
		if stat.Nullified {
			nullified = true
			break
		}
	}

	// Map API period type to database enum value
	periodType, err := MapPeriodTypeToDB(period.PeriodType)
	if err != nil {
		return fmt.Errorf("failed to map period type: %w", err)
	}

	// Upsert the play
	play := &store.NFLPlayForUpsert{
		DriveID:                driveID,
		VendorID:               event.ID,
		Sequence:               decimal.NewFromFloat(event.Sequence),
		PeriodType:             periodType,
		PeriodNumber:           period.Number,
		Description:            event.Description,
		AlternativeDescription: event.AltDescription,
		Nullified:              nullified,
		VendorCreatedAt:        createdAt,
		VendorUpdatedAt:        updatedAt,
	}

	playID, err := dbStore.UpsertNFLPlay(ctx, tx, play)
	if err != nil {
		return err
	}

	// Prepare statistics for upsert
	var stats []*store.PlayStatisticForUpsert
	for _, stat := range event.Statistics {
		// Skip statistics without a player (team-level stats)
		if !shouldPersistPlayStatistic(&stat) {
			continue
		}

		// Map API stat type to database enum value
		statType, err := MapStatTypeToDB(stat.StatType)
		if err != nil {
			return fmt.Errorf("failed to map stat type: %w", err)
		}

		// Create the base statistic struct
		playStatistic := &store.PlayStatisticForUpsert{
			VendorPlayerID: stat.Player.ID, // This is vendor_id, not db id
			StatType:       statType,
			Nullified:      stat.Nullified,
			// Initialize all stat-type-specific fields to 0
			PassingAttempts:     decimal.NewFromFloat(0),
			RushingAttempts:     decimal.NewFromFloat(0),
			ReceivingTargets:    decimal.NewFromFloat(0),
			PassingYards:        decimal.NewFromFloat(0),
			RushingYards:        decimal.NewFromFloat(0),
			ReceivingYards:      decimal.NewFromFloat(0),
			PassingTouchdowns:   decimal.NewFromFloat(0),
			RushingTouchdowns:   decimal.NewFromFloat(0),
			ReceivingTouchdowns: decimal.NewFromFloat(0),
			Completions:         decimal.NewFromFloat(stat.Complete),
			Incompletions:       decimal.NewFromFloat(stat.Incomplete),
			Receptions:          decimal.NewFromFloat(stat.Reception),
			InterceptionsThrown: decimal.NewFromFloat(0),
			Interceptions:       decimal.NewFromFloat(0),
			Fumbles:             decimal.NewFromFloat(0),
			FumblesLost:         decimal.NewFromFloat(0),
			SacksTaken:          decimal.NewFromFloat(0),
			Sacks:               decimal.NewFromFloat(0),
			Tackles:             decimal.NewFromFloat(stat.Tackle),
			Assists:             decimal.NewFromFloat(stat.Assist),
		}

		// Populate stat-type-specific fields based on the stat type
		switch statType {
		case "passing":
			playStatistic.PassingAttempts = decimal.NewFromFloat(stat.Attempt)
			playStatistic.PassingYards = decimal.NewFromFloat(stat.Yards)
			playStatistic.PassingTouchdowns = decimal.NewFromFloat(stat.Touchdown)
			playStatistic.InterceptionsThrown = decimal.NewFromFloat(stat.Interception)
			playStatistic.SacksTaken = decimal.NewFromFloat(stat.Sack)
		case "rushing":
			playStatistic.RushingAttempts = decimal.NewFromFloat(stat.Attempt)
			playStatistic.RushingYards = decimal.NewFromFloat(stat.Yards)
			playStatistic.RushingTouchdowns = decimal.NewFromFloat(stat.Touchdown)
		case "receiving":
			playStatistic.ReceivingTargets = decimal.NewFromFloat(stat.Target)
			playStatistic.ReceivingYards = decimal.NewFromFloat(stat.Yards)
			playStatistic.ReceivingTouchdowns = decimal.NewFromFloat(stat.Touchdown)
		case "defense":
			playStatistic.Sacks = decimal.NewFromFloat(stat.Sack)
			// Tackles and Assists already set above
		case "interception":
			playStatistic.Interceptions = decimal.NewFromFloat(stat.Interception)
		case "fumble":
			// API doesn't provide fumble details at stat level, keeping as 0
			// playStatistic.Fumbles and FumblesLost remain 0
		case "field_goal", "extra_point":
			// These stat types don't have yards/attempts/touchdowns in our schema
		default:
			return fmt.Errorf("unexpected stat type after mapping: %s", statType)
		}

		stats = append(stats, playStatistic)
	}

	// Replace all statistics for this play - player lookups done via subqueries
	if err := dbStore.ReplaceNFLPlayStatistics(ctx, tx, playID, stats); err != nil {
		return fmt.Errorf("failed to replace statistics: %w", err)
	}

	return nil
}

// parseTimestamp parses a Sportradar timestamp string (ISO 8601 format)
func parseTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	// Sportradar uses ISO 8601 format: "2024-12-26T15:30:00+00:00"
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format %q: %w", ts, err)
	}
	return t, nil
}
