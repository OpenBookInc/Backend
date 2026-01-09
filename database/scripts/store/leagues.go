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

// GetLeagueByName retrieves a league by name
func (s *Store) GetLeagueByName(ctx context.Context, name string) (*models.League, error) {
	query := `SELECT id, sport_id, name FROM leagues WHERE name = $1`

	var league models.League
	err := s.pool.QueryRow(ctx, query, name).Scan(&league.ID, &league.SportID, &league.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get league %s: %w", name, err)
	}

	return &league, nil
}
