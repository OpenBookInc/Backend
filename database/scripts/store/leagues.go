package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// LeagueForUpsert contains the data needed to upsert a league
type LeagueForUpsert struct {
	SportID int
	Name    string
}

// UpsertLeague inserts or updates a league in the database
// Uses name as the unique identifier (ON CONFLICT)
// Returns the database ID of the league
func (s *Store) UpsertLeague(ctx context.Context, league *LeagueForUpsert) (int, error) {
	query := `
		INSERT INTO leagues (sport_id, name)
		VALUES ($1, $2)
		ON CONFLICT (name)
		DO UPDATE SET sport_id = EXCLUDED.sport_id
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query, league.SportID, league.Name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert league %s: %w", league.Name, err)
	}

	return id, nil
}

// GetLeagueByID retrieves a league by database ID.
// Uses the registry for caching: checks registry first, fetches from DB if not found.
func (s *Store) GetLeagueByID(ctx context.Context, id int) (*models.League, error) {
	// Check registry first
	if league := models.Registry.GetLeague(id); league != nil {
		return league, nil
	}

	// Query database
	query := `SELECT id, sport_id, name FROM leagues WHERE id = $1`

	var league models.League
	err := s.pool.QueryRow(ctx, query, id).Scan(&league.ID, &league.SportID, &league.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get league with id %d: %w", id, err)
	}

	// Register and return
	return models.Registry.RegisterLeague(&league), nil
}

// GetLeagueByName retrieves a league by name.
// Uses the registry for caching: checks registry first, fetches from DB if not found.
func (s *Store) GetLeagueByName(ctx context.Context, name string) (*models.League, error) {
	// Query database to get the ID first (no registry lookup by name)
	query := `SELECT id, sport_id, name FROM leagues WHERE name = $1`

	var league models.League
	err := s.pool.QueryRow(ctx, query, name).Scan(&league.ID, &league.SportID, &league.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get league %s: %w", name, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetLeague(league.ID); existing != nil {
		return existing, nil
	}

	// Register and return
	return models.Registry.RegisterLeague(&league), nil
}
