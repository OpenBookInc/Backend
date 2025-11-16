package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models"
)

// UpsertDivision inserts or updates a division in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the division
func (s *Store) UpsertDivision(ctx context.Context, division *models.Division) (int, error) {
	query := `
		INSERT INTO divisions (name, conference_id, vendor_id, alias)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			conference_id = EXCLUDED.conference_id,
			alias = EXCLUDED.alias
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		division.Name,
		division.ConferenceID,
		division.VendorID,
		division.Alias,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert division %s (vendor_id: %s): %w",
			division.Name, division.VendorID, err)
	}

	return id, nil
}

// GetDivisionByVendorID retrieves a division by vendor_id
func (s *Store) GetDivisionByVendorID(ctx context.Context, vendorID string) (*models.Division, error) {
	query := `SELECT id, name, conference_id, vendor_id, alias FROM divisions WHERE vendor_id = $1`

	var division models.Division
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&division.ID,
		&division.Name,
		&division.ConferenceID,
		&division.VendorID,
		&division.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get division with vendor_id %s: %w", vendorID, err)
	}

	return &division, nil
}
