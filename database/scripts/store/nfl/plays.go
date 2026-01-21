package nfl

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
	"github.com/shopspring/decimal"
)

// NFLPlayForUpsert contains the data needed to upsert an NFL play
type NFLPlayForUpsert struct {
	DriveID         int
	VendorID        string
	VendorSequence  decimal.Decimal
	PeriodType      string // DB enum value as string (e.g., "quarter", "overtime")
	PeriodNumber    int
	Description     string
	Nullified       bool
	VendorCreatedAt time.Time
	VendorUpdatedAt time.Time
}

// UpsertNFLPlay inserts or updates an NFL play in the database.
// Uses (drive_id, vendor_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the play for use as a foreign key in statistics.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
// Sets vendor_deleted = FALSE on both insert and update.
func UpsertNFLPlay(s *store.Store, ctx context.Context, tx pgx.Tx, play *NFLPlayForUpsert) (int, error) {
	query := `
		INSERT INTO nfl_plays (
			drive_id, vendor_id, vendor_sequence,
			period_type, period_number,
			description, nullified,
			vendor_deleted, vendor_created_at, vendor_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::period_type, $5, $6, $7, FALSE, $8, $9, NOW(), NOW())
		ON CONFLICT (drive_id, vendor_id)
		DO UPDATE SET
			vendor_sequence = EXCLUDED.vendor_sequence,
			period_type = EXCLUDED.period_type,
			period_number = EXCLUDED.period_number,
			description = EXCLUDED.description,
			nullified = EXCLUDED.nullified,
			vendor_deleted = FALSE,
			vendor_created_at = EXCLUDED.vendor_created_at,
			vendor_updated_at = EXCLUDED.vendor_updated_at,
			updated_at = NOW()
		RETURNING id
	`

	var id int
	err := tx.QueryRow(ctx, query,
		play.DriveID,
		play.VendorID,
		play.VendorSequence,
		play.PeriodType,
		play.PeriodNumber,
		play.Description,
		play.Nullified,
		play.VendorCreatedAt,
		play.VendorUpdatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NFL play (vendor_id: %s): %w", play.VendorID, err)
	}

	return id, nil
}

// GetNFLPlayByVendorID retrieves an NFL play by drive_id and vendor_id.
// Only returns plays where vendor_deleted = FALSE and the parent drive is not deleted.
func GetNFLPlayByVendorID(s *store.Store, ctx context.Context, driveID int, vendorID string) (*models_nfl.Play, error) {
	query := `
		SELECT p.id, p.drive_id, p.vendor_id, p.vendor_sequence,
		       p.period_type, p.period_number,
		       p.description, p.nullified,
		       p.vendor_created_at, p.vendor_updated_at, p.created_at, p.updated_at
		FROM nfl_plays p
		JOIN nfl_drives d ON p.drive_id = d.id
		WHERE p.drive_id = $1 AND p.vendor_id = $2
		  AND p.vendor_deleted = FALSE AND d.vendor_deleted = FALSE
	`

	var play models_nfl.Play
	err := s.Pool().QueryRow(ctx, query, driveID, vendorID).Scan(
		&play.ID,
		&play.DriveID,
		&play.VendorID,
		&play.VendorSequence,
		&play.PeriodType,
		&play.PeriodNumber,
		&play.Description,
		&play.Nullified,
		&play.VendorCreatedAt,
		&play.VendorUpdatedAt,
		&play.CreatedAt,
		&play.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NFL play with vendor_id %s: %w", vendorID, err)
	}

	return &play, nil
}

// MarkNFLPlayDeleted marks an NFL play as vendor_deleted = TRUE.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func MarkNFLPlayDeleted(s *store.Store, ctx context.Context, tx pgx.Tx, playID int) error {
	query := `
		UPDATE nfl_plays
		SET vendor_deleted = TRUE, updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, playID)
	if err != nil {
		return fmt.Errorf("failed to mark play as deleted (play_id: %d): %w", playID, err)
	}

	return nil
}
