package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	nflmodels "github.com/openbook/shared/models/nfl"
)

// UpsertNFLPlay inserts or updates an NFL play in the database.
// Uses (drive_id, vendor_id) as the unique constraint for ON CONFLICT.
// Returns the database ID of the play for use as a foreign key in statistics.
// This function accepts a transaction (pgx.Tx) to support atomic operations.
func (s *Store) UpsertNFLPlay(ctx context.Context, tx pgx.Tx, play *nflmodels.Play) (int, error) {
	query := `
		INSERT INTO nfl_plays (
			drive_id, vendor_id, sequence,
			period_type, period_number,
			description, alternative_description, nullified,
			vendor_created_at, vendor_updated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::nfl_period_type, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		ON CONFLICT (drive_id, vendor_id)
		DO UPDATE SET
			sequence = EXCLUDED.sequence,
			period_type = EXCLUDED.period_type,
			period_number = EXCLUDED.period_number,
			description = EXCLUDED.description,
			alternative_description = EXCLUDED.alternative_description,
			nullified = EXCLUDED.nullified,
			vendor_created_at = EXCLUDED.vendor_created_at,
			vendor_updated_at = EXCLUDED.vendor_updated_at,
			updated_at = NOW()
		RETURNING id
	`

	var id int
	err := tx.QueryRow(ctx, query,
		play.DriveID,
		play.VendorID,
		play.Sequence,
		string(play.PeriodType),
		play.PeriodNumber,
		play.Description,
		play.AlternativeDescription,
		play.Nullified,
		play.VendorCreatedAt,
		play.VendorUpdatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert NFL play (vendor_id: %s): %w", play.VendorID, err)
	}

	return id, nil
}

// GetNFLPlayByVendorID retrieves an NFL play by drive_id and vendor_id
func (s *Store) GetNFLPlayByVendorID(ctx context.Context, driveID int, vendorID string) (*nflmodels.Play, error) {
	query := `
		SELECT id, drive_id, vendor_id, sequence,
		       period_type, period_number,
		       description, alternative_description, nullified,
		       vendor_created_at, vendor_updated_at, created_at, updated_at
		FROM nfl_plays
		WHERE drive_id = $1 AND vendor_id = $2
	`

	var play nflmodels.Play
	err := s.pool.QueryRow(ctx, query, driveID, vendorID).Scan(
		&play.ID,
		&play.DriveID,
		&play.VendorID,
		&play.Sequence,
		&play.PeriodType,
		&play.PeriodNumber,
		&play.Description,
		&play.AlternativeDescription,
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
