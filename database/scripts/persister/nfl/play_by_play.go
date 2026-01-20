package nfl

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openbook/population-scripts/client/sportradar"
	fetcher_nfl "github.com/openbook/population-scripts/fetcher/nfl"
	"github.com/openbook/population-scripts/persister"
	"github.com/openbook/population-scripts/store"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
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
//
// Before persisting play-by-play data, this function ensures all referenced players
// exist in the database by fetching and persisting any missing player profiles.
func PersistNFLPlayByPlay(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, gameID int, pbp *fetcher_nfl.PlayByPlayResponse) error {
	// Step 1: Ensure all referenced players exist in the database
	if err := persistMissingIndividuals(ctx, dbStore, apiClient, pbp); err != nil {
		return fmt.Errorf("failed to persist missing individuals: %w", err)
	}

	// Step 2: Start transaction for all write operations
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

	// Step 3: Upsert game status
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

	// Step 4: Process all drives, plays, and statistics
	for _, period := range pbp.Periods {
		for _, drive := range period.PBP {
			if err := persistDrive(ctx, dbStore, tx, gameID, &period, &drive); err != nil {
				return fmt.Errorf("failed to persist drive (vendor_id: %s): %w", drive.ID, err)
			}
		}
	}

	// Step 5: Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Mark tx as nil so deferred rollback is a no-op
	tx = nil

	return nil
}

// persistDrive upserts a drive and all its plays within the transaction.
// Only processes drives with type "drive" or "play" (skips "event" entries like timeouts).
// Foreign keys are resolved via subqueries in the store layer.
func persistDrive(ctx context.Context, dbStore *store.Store, tx pgx.Tx, gameID int, period *fetcher_nfl.Period, drive *fetcher_nfl.Drive) error {
	// Skip non-drive entries (e.g., type "event" for timeouts, end-of-period markers)
	if !shouldPersistDrive(drive) {
		return nil
	}

	// Determine the possession team vendor ID.
	// For regular drives (type="drive"), use the offensive_team field.
	// For standalone plays (type="play", play_type="extra_point" or "conversion"),
	// the offensive_team is nil because the API structures these differently - they
	// occur after special teams touchdowns (punt/kick returns) where there's no
	// traditional "offensive drive". In this case, we use start_situation.possession
	// which indicates the team attempting the PAT or two-point conversion.
	var vendorOffensiveTeamID string
	if drive.OffensiveTeam != nil {
		vendorOffensiveTeamID = drive.OffensiveTeam.ID
	} else if drive.IsStandalonePlayDrive() {
		// Standalone PAT or two-point conversion after a special teams touchdown.
		// The offensive_team is nil because this isn't a traditional drive, but the
		// team attempting the scoring play is available in start_situation.possession.
		if drive.StartSituation == nil || drive.StartSituation.Possession == nil {
			return fmt.Errorf("standalone play missing start_situation.possession information")
		}
		vendorOffensiveTeamID = drive.StartSituation.Possession.ID
	} else {
		return fmt.Errorf("drive missing offensive team information")
	}

	// Upsert the drive - team lookup done via subquery
	driveID, err := store_nfl.UpsertNFLDrive(dbStore, 
		ctx,
		tx,
		gameID,
		drive.ID,
		decimal.NewFromFloat(drive.Sequence),
		vendorOffensiveTeamID,
	)
	if err != nil {
		return err
	}

	// For standalone play entries (e.g., extra points or two-point conversions after
	// special teams TDs), the play data and statistics are at the drive level, not
	// nested in events. Convert the drive to an Event and persist it using the standard flow.
	if drive.IsStandalonePlayDrive() {
		event := drive.AsEvent()
		if err := persistPlay(ctx, dbStore, tx, driveID, period, event); err != nil {
			return fmt.Errorf("failed to persist standalone play (vendor_id: %s): %w", drive.ID, err)
		}
		return nil
	}

	// Defensive assertion: For non-standalone-play drives, statistics should be
	// nested in events, not at the drive level. If we find drive-level statistics
	// on a regular drive, it indicates an unexpected API structure that we should
	// investigate rather than silently ignore.
	if len(drive.Statistics) > 0 {
		return fmt.Errorf("unexpected drive-level statistics on non-standalone-play drive (id: %s, type: %s, play_type: %s)", drive.ID, drive.Type, drive.PlayType)
	}

	// Process each event in the drive (for regular drives with nested events)
	for _, event := range drive.Events {
		if err := persistPlay(ctx, dbStore, tx, driveID, period, &event); err != nil {
			return fmt.Errorf("failed to persist play (vendor_id: %s): %w", event.ID, err)
		}
	}

	return nil
}

// persistPlay upserts a play and all its statistics within the transaction.
// Foreign keys are resolved via subqueries in the store layer.
func persistPlay(ctx context.Context, dbStore *store.Store, tx pgx.Tx, driveID int, period *fetcher_nfl.Period, event *fetcher_nfl.Event) error {
	if !shouldPersistPlay(event) {
		return nil
	}

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
	play := &store_nfl.NFLPlayForUpsert{
		DriveID:         driveID,
		VendorID:        event.ID,
		VendorSequence:  decimal.NewFromFloat(event.Sequence),
		PeriodType:      periodType,
		PeriodNumber:    period.Number,
		Description:     event.Description,
		Nullified:       nullified,
		VendorCreatedAt: createdAt,
		VendorUpdatedAt: updatedAt,
	}

	playID, err := store_nfl.UpsertNFLPlay(dbStore, ctx, tx, play)
	if err != nil {
		return err
	}

	// Prepare statistics for upsert
	var stats []*store_nfl.PlayStatisticForUpsert
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
		playStatistic := &store_nfl.PlayStatisticForUpsert{
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
			PassingCompletions:  decimal.NewFromFloat(stat.Complete),
			ReceivingReceptions: decimal.NewFromFloat(stat.Reception),
			InterceptionsThrown: decimal.NewFromFloat(0),
			InterceptionsCaught: decimal.NewFromFloat(0),
			FumblesCommitted:    decimal.NewFromFloat(0),
			SacksTaken:          decimal.NewFromFloat(0),
			SacksMade:           decimal.NewFromFloat(0),
			SackAssistsMade:     decimal.NewFromFloat(stat.AstSack),
			FieldGoalAttempts:   decimal.NewFromFloat(0),
			FieldGoalMakes:      decimal.NewFromFloat(0),
			ExtraPointAttempts:  decimal.NewFromFloat(0),
			ExtraPointMakes:     decimal.NewFromFloat(0),
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
			playStatistic.SacksMade = decimal.NewFromFloat(stat.Sack)
			playStatistic.InterceptionsCaught = decimal.NewFromFloat(stat.Interception)
		case "fumble":
			playStatistic.FumblesCommitted = decimal.NewFromFloat(stat.Fumble)
		case "field_goal":
			playStatistic.FieldGoalAttempts = decimal.NewFromFloat(stat.Attempt)
			playStatistic.FieldGoalMakes = decimal.NewFromFloat(stat.Made)
		case "extra_point":
			playStatistic.ExtraPointAttempts = decimal.NewFromFloat(stat.Attempt)
			playStatistic.ExtraPointMakes = decimal.NewFromFloat(stat.Made)
		case "miscellaneous":
			// No stat-type-specific fields to populate for miscellaneous stats
		default:
			return fmt.Errorf("unexpected stat type after mapping: %s", statType)
		}

		if !shouldPersistPlayStatisticForUpsert(playStatistic) {
			continue
		}

		stats = append(stats, playStatistic)
	}

	// Replace all statistics for this play - player lookups done via subqueries
	if err := store_nfl.ReplaceNFLPlayStatistics(dbStore, ctx, tx, playID, stats); err != nil {
		return fmt.Errorf("failed to replace statistics: %w", err)
	}

	return nil
}

// persistMissingIndividuals ensures all players referenced in the play-by-play data
// exist in the database. For any missing players, it fetches their profile from the
// Sportradar API and persists them.
func persistMissingIndividuals(ctx context.Context, dbStore *store.Store, apiClient *sportradar.Client, pbp *fetcher_nfl.PlayByPlayResponse) error {
	// Extract all unique player vendor IDs from persistable statistics
	playerVendorIDs := ExtractPlayerVendorIDs(pbp)

	// Check each player and fetch/persist if missing
	for _, vendorID := range playerVendorIDs {
		individual, created, err := persister.UpsertIndividualIfMissing(ctx, dbStore, apiClient, vendorID, "NFL")
		if err != nil {
			return fmt.Errorf("failed to ensure player exists (vendor_id: %s): %w", vendorID, err)
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
	// Sportradar uses ISO 8601 format: "2024-12-26T15:30:00+00:00"
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format %q: %w", ts, err)
	}
	return t, nil
}
