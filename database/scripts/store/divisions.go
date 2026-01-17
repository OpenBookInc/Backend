package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// DivisionForUpsert contains the data needed to upsert a division
type DivisionForUpsert struct {
	VendorID     string
	Name         string
	ConferenceID int
	Alias        string
}

// UpsertDivision inserts or updates a division in the database
// Uses vendor_id as the unique identifier (ON CONFLICT)
// Returns the database ID of the division
func (s *Store) UpsertDivision(ctx context.Context, division *DivisionForUpsert) (int, error) {
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

// GetDivisionByID retrieves a division by database ID.
// Uses the registry for caching and resolves the nested Conference pointer.
func (s *Store) GetDivisionByID(ctx context.Context, id int) (*models.Division, error) {
	// Check registry first
	if division := models.Registry.GetDivision(id); division != nil {
		return division, nil
	}

	// Query database
	query := `SELECT id, name, conference_id, vendor_id, alias FROM divisions WHERE id = $1`

	var division models.Division
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&division.ID,
		&division.Name,
		&division.ConferenceID,
		&division.VendorID,
		&division.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get division with id %d: %w", id, err)
	}

	// Resolve nested Conference pointer
	conference, err := s.GetConferenceByID(ctx, int(division.ConferenceID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conference for division %d: %w", id, err)
	}
	division.Conference = conference

	// Register and return
	return models.Registry.RegisterDivision(&division), nil
}

// GetDivisionByVendorID retrieves a division by vendor_id.
// Uses the registry for caching and resolves the nested Conference pointer.
func (s *Store) GetDivisionByVendorID(ctx context.Context, vendorID string) (*models.Division, error) {
	// Query database to get the ID first
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

	// Check if already registered (by ID)
	if existing := models.Registry.GetDivision(division.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Conference pointer
	conference, err := s.GetConferenceByID(ctx, int(division.ConferenceID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conference for division %s: %w", vendorID, err)
	}
	division.Conference = conference

	// Register and return
	return models.Registry.RegisterDivision(&division), nil
}
