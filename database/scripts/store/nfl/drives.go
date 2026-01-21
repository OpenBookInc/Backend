package nfl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
)

// UpsertNFLDrive inserts or updates an NFL drive in the database.
// Uses (game_id, vendor_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the drive for use as a foreign key in plays.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// The vendorTeamID is looked up via subquery to get the possession_team_id.
// Sets vendor_deleted = FALSE on both insert and update.
func UpsertNFLDrive(s *store.Store, ctx context.Context, tx pgx.Tx, gameID int, vendorDriveID string, sequence interface{}, vendorTeamID string) (int, error) {
	query := `
		INSERT INTO nfl_drives (
			game_id, vendor_id, vendor_sequence, possession_team_id,
			vendor_deleted, created_at, updated_at
		)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT id FROM teams WHERE vendor_id = $4),
			FALSE,
			NOW(),
			NOW()
		)
		ON CONFLICT (game_id, vendor_id)
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
		vendorDriveID,
		sequence,
		vendorTeamID,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NFL drive (vendor_id: %s, vendor_team_id: %s - team may not exist in database): %w", vendorDriveID, vendorTeamID, err)
	}

	return id, nil
}

// GetNFLDriveByVendorID retrieves an NFL drive by game_id and vendor_id.
// Only returns drives where vendor_deleted = FALSE.
func GetNFLDriveByVendorID(s *store.Store, ctx context.Context, gameID int, vendorID string) (*models_nfl.Drive, error) {
	query := `
		SELECT id, game_id, vendor_id, vendor_sequence, possession_team_id,
		       created_at, updated_at
		FROM nfl_drives
		WHERE game_id = $1 AND vendor_id = $2 AND vendor_deleted = FALSE
	`

	var drive models_nfl.Drive
	err := s.Pool().QueryRow(ctx, query, gameID, vendorID).Scan(
		&drive.ID,
		&drive.GameID,
		&drive.VendorID,
		&drive.VendorSequence,
		&drive.PossessionTeamID,
		&drive.CreatedAt,
		&drive.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL drive with vendor_id %s: %w", vendorID, err)
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
