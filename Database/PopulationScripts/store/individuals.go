package store

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/models"
)

// UpsertIndividual inserts or updates an individual in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the individual
func (s *Store) UpsertIndividual(ctx context.Context, individual *models.Individual) (int, error) {
	query := `
		INSERT INTO individuals (display_name, abbreviated_name, date_of_birth, vendor_id, league_id, position, jersey_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (vendor_id)
		DO UPDATE SET
			display_name = EXCLUDED.display_name,
			abbreviated_name = EXCLUDED.abbreviated_name,
			date_of_birth = EXCLUDED.date_of_birth,
			league_id = EXCLUDED.league_id,
			position = EXCLUDED.position,
			jersey_number = EXCLUDED.jersey_number
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		individual.DisplayName,
		individual.AbbreviatedName,
		individual.DateOfBirth, // Can be nil for NULL
		individual.VendorID,
		individual.LeagueID,
		individual.Position,
		individual.JerseyNumber,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert individual %s (vendor_id: %s): %w",
			individual.DisplayName, individual.VendorID, err)
	}

	return id, nil
}

// GetIndividualByVendorID retrieves an individual by vendor_id
func (s *Store) GetIndividualByVendorID(ctx context.Context, vendorID string) (*models.Individual, error) {
	query := `
		SELECT id, display_name, abbreviated_name, date_of_birth, vendor_id, league_id, position, jersey_number
		FROM individuals
		WHERE vendor_id = $1
	`

	var individual models.Individual
	err := s.pool.QueryRow(ctx, query, vendorID).Scan(
		&individual.ID,
		&individual.DisplayName,
		&individual.AbbreviatedName,
		&individual.DateOfBirth,
		&individual.VendorID,
		&individual.LeagueID,
		&individual.Position,
		&individual.JerseyNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get individual with vendor_id %s: %w", vendorID, err)
	}

	return &individual, nil
}
