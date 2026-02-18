package store

import (
	"context"
	"fmt"
	"time"

	models "github.com/openbook/shared/models"
)

// IndividualForUpsert contains the data needed to upsert an individual
type IndividualForUpsert struct {
	VendorID        string
	DisplayName     string
	AbbreviatedName string
	DateOfBirth     *time.Time
	LeagueID        int
	Position        string
	JerseyNumber    string
}

// UpsertIndividual inserts or updates an individual in the database.
// Uses vendor_id as the unique identifier (ON CONFLICT).
// Resolves the League pointer, registers in the singleton registry, and returns the individual.
func (s *Store) UpsertIndividual(ctx context.Context, individual *IndividualForUpsert) (*models.Individual, error) {
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
		return nil, fmt.Errorf("failed to upsert individual %s (vendor_id: %s): %w",
			individual.DisplayName, individual.VendorID, err)
	}

	league, err := s.GetLeagueByID(ctx, individual.LeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %s: %w", individual.VendorID, err)
	}

	return models.Registry.RegisterIndividual(&models.Individual{
		ID:              id,
		DisplayName:     individual.DisplayName,
		AbbreviatedName: individual.AbbreviatedName,
		DateOfBirth:     individual.DateOfBirth,
		VendorID:        individual.VendorID,
		LeagueID:        int64(individual.LeagueID),
		Position:        individual.Position,
		JerseyNumber:    individual.JerseyNumber,
		League:          league,
	}), nil
}

// GetIndividualByID retrieves an individual by database ID.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetIndividualByID(ctx context.Context, id int) (*models.Individual, error) {
	// Check registry first
	if individual := models.Registry.GetIndividual(id); individual != nil {
		return individual, nil
	}

	// Query database
	query := `
		SELECT id, display_name, abbreviated_name, date_of_birth, vendor_id, league_id, position, jersey_number
		FROM individuals
		WHERE id = $1
	`

	var individual models.Individual
	err := s.pool.QueryRow(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("failed to get individual with id %d: %w", id, err)
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(individual.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %d: %w", id, err)
	}
	individual.League = league

	// Register and return
	return models.Registry.RegisterIndividual(&individual), nil
}

// GetIndividualByVendorID retrieves an individual by vendor_id.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetIndividualByVendorID(ctx context.Context, vendorID string) (*models.Individual, error) {
	// Query database to get the ID first
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

	// Check if already registered (by ID)
	if existing := models.Registry.GetIndividual(individual.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(individual.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for individual %s: %w", vendorID, err)
	}
	individual.League = league

	// Register and return
	return models.Registry.RegisterIndividual(&individual), nil
}
