package nfl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
)

// UpsertNFLDrive inserts or updates an NFL drive in the database.
// Uses (game_id, sportradar_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the drive for use as a foreign key in plays.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// The sportradarTeamID is looked up via subquery to get the possession_team_id.
// Sets vendor_deleted = FALSE on both insert and update.
func UpsertNFLDrive(s *store.Store, ctx context.Context, tx pgx.Tx, gameID string, sportradarDriveID string, sequence interface{}, sportradarTeamID string) (int, error) {
	query := `
		INSERT INTO nfl_drives (
			game_id, sportradar_id, vendor_sequence, possession_team_id,
			vendor_deleted, created_at, updated_at
		)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT id FROM teams WHERE sportradar_id = $4),
			FALSE,
			NOW(),
			NOW()
		)
		ON CONFLICT (game_id, sportradar_id)
		DO UPDATE SET
			vendor_sequence = EXCLUDED.vendor_sequence,
			possession_team_id = EXCLUDED.possession_team_id,
			vendor_deleted = FALSE,
			updated_at = NOW()
		RETURNING id
	`

	var id int
	err := tx.QueryRow(ctx, query,
		gameID,
		sportradarDriveID,
		sequence,
		sportradarTeamID,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NFL drive (sportradar_id: %s, sportradar_team_id: %s - team may not exist in database): %w", sportradarDriveID, sportradarTeamID, err)
	}

	return id, nil
}

// GetNFLDriveBySportradarID retrieves an NFL drive by game_id and sportradar_id.
// Only returns drives where vendor_deleted = FALSE.
func GetNFLDriveBySportradarID(s *store.Store, ctx context.Context, gameID string, sportradarID string) (*models_nfl.Drive, error) {
	query := `
		SELECT id, game_id, sportradar_id, vendor_sequence, possession_team_id,
		       created_at, updated_at
		FROM nfl_drives
		WHERE game_id = $1 AND sportradar_id = $2 AND vendor_deleted = FALSE
	`

	var drive models_nfl.Drive
	err := s.Pool().QueryRow(ctx, query, gameID, sportradarID).Scan(
		&drive.ID,
		&drive.GameID,
		&drive.SportradarID,
		&drive.VendorSequence,
		&drive.PossessionTeamID,
		&drive.CreatedAt,
		&drive.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL drive with sportradar_id %s: %w", sportradarID, err)
	}

	return &drive, nil
}

// MarkNFLDriveDeleted marks an NFL drive as vendor_deleted = TRUE.
// Also cascades to mark all associated plays as deleted.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func MarkNFLDriveDeleted(s *store.Store, ctx context.Context, tx pgx.Tx, driveID int) error {
	// First, mark all plays associated with this drive as deleted
	playQuery := `
		UPDATE nfl_plays
		SET vendor_deleted = TRUE, updated_at = NOW()
		WHERE drive_id = $1
	`
	_, err := tx.Exec(ctx, playQuery, driveID)
	if err != nil {
		return fmt.Errorf("failed to mark plays as deleted for drive_id %d: %w", driveID, err)
	}

	// Then mark the drive itself as deleted
	driveQuery := `
		UPDATE nfl_drives
		SET vendor_deleted = TRUE, updated_at = NOW()
		WHERE id = $1
	`
	_, err = tx.Exec(ctx, driveQuery, driveID)
	if err != nil {
		return fmt.Errorf("failed to mark drive as deleted (drive_id: %d): %w", driveID, err)
	}

	return nil
}
