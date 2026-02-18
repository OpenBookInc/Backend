package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// DivisionForUpsert contains the data needed to upsert a division
type DivisionForUpsert struct {
	SportradarID string
	Name         string
	ConferenceID int
	Alias        string
}

// UpsertDivision inserts or updates a division in the database.
// Uses sportradar_id as the unique identifier (ON CONFLICT).
// Resolves the Conference pointer, registers in the singleton registry, and returns the division.
func (s *Store) UpsertDivision(ctx context.Context, division *DivisionForUpsert) (*models.Division, error) {
	query := `
		INSERT INTO divisions (name, conference_id, sportradar_id, alias)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sportradar_id)
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
		division.SportradarID,
		division.Alias,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert division %s (sportradar_id: %s): %w",
			division.Name, division.SportradarID, err)
	}

	conference, err := s.GetConferenceByID(ctx, division.ConferenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conference for division %s: %w", division.SportradarID, err)
	}

	return models.Registry.RegisterDivision(&models.Division{
		ID:           id,
		Name:         division.Name,
		ConferenceID: int64(division.ConferenceID),
		SportradarID: division.SportradarID,
		Alias:        division.Alias,
		Conference:   conference,
	}), nil
}

// GetDivisionByID retrieves a division by database ID.
// Uses the registry for caching and resolves the nested Conference pointer.
func (s *Store) GetDivisionByID(ctx context.Context, id int) (*models.Division, error) {
	// Check registry first
	if division := models.Registry.GetDivision(id); division != nil {
		return division, nil
	}

	// Query database
	query := `SELECT id, name, conference_id, sportradar_id, alias FROM divisions WHERE id = $1`

	var division models.Division
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&division.ID,
		&division.Name,
		&division.ConferenceID,
		&division.SportradarID,
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

// GetDivisionBySportradarID retrieves a division by sportradar_id.
// Uses the registry for caching and resolves the nested Conference pointer.
func (s *Store) GetDivisionBySportradarID(ctx context.Context, sportradarID string) (*models.Division, error) {
	// Query database to get the ID first
	query := `SELECT id, name, conference_id, sportradar_id, alias FROM divisions WHERE sportradar_id = $1`

	var division models.Division
	err := s.pool.QueryRow(ctx, query, sportradarID).Scan(
		&division.ID,
		&division.Name,
		&division.ConferenceID,
		&division.SportradarID,
		&division.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get division with sportradar_id %s: %w", sportradarID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetDivision(division.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Conference pointer
	conference, err := s.GetConferenceByID(ctx, int(division.ConferenceID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conference for division %s: %w", sportradarID, err)
	}
	division.Conference = conference

	// Register and return
	return models.Registry.RegisterDivision(&division), nil
}
