package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// ConferenceForUpsert contains the data needed to upsert a conference
type ConferenceForUpsert struct {
	SportradarID string
	Name         string
	LeagueID     int
	Alias        string
}

// UpsertConference inserts or updates a conference in the database.
// Uses sportradar_id as the unique identifier (ON CONFLICT).
// Resolves the League pointer, registers in the singleton registry, and returns the conference.
func (s *Store) UpsertConference(ctx context.Context, conference *ConferenceForUpsert) (*models.Conference, error) {
	query := `
		INSERT INTO conferences (name, league_id, sportradar_id, alias)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sportradar_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			league_id = EXCLUDED.league_id,
			alias = EXCLUDED.alias
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query,
		conference.Name,
		conference.LeagueID,
		conference.SportradarID,
		conference.Alias,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert conference %s (sportradar_id: %s): %w",
			conference.Name, conference.SportradarID, err)
	}

	league, err := s.GetLeagueByID(ctx, conference.LeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for conference %s: %w", conference.SportradarID, err)
	}

	return models.Registry.RegisterConference(&models.Conference{
		ID:           id,
		Name:         conference.Name,
		LeagueID:     int64(conference.LeagueID),
		SportradarID: conference.SportradarID,
		Alias:        conference.Alias,
		League:       league,
	}), nil
}

// GetConferenceByID retrieves a conference by database ID.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetConferenceByID(ctx context.Context, id int) (*models.Conference, error) {
	// Check registry first
	if conference := models.Registry.GetConference(id); conference != nil {
		return conference, nil
	}

	// Query database
	query := `SELECT id, name, league_id, sportradar_id, alias FROM conferences WHERE id = $1`

	var conference models.Conference
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&conference.ID,
		&conference.Name,
		&conference.LeagueID,
		&conference.SportradarID,
		&conference.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get conference with id %d: %w", id, err)
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(conference.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for conference %d: %w", id, err)
	}
	conference.League = league

	// Register and return
	return models.Registry.RegisterConference(&conference), nil
}

// GetConferenceBySportradarID retrieves a conference by sportradar_id.
// Uses the registry for caching and resolves the nested League pointer.
func (s *Store) GetConferenceBySportradarID(ctx context.Context, sportradarID string) (*models.Conference, error) {
	// Query database to get the ID first
	query := `SELECT id, name, league_id, sportradar_id, alias FROM conferences WHERE sportradar_id = $1`

	var conference models.Conference
	err := s.pool.QueryRow(ctx, query, sportradarID).Scan(
		&conference.ID,
		&conference.Name,
		&conference.LeagueID,
		&conference.SportradarID,
		&conference.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get conference with sportradar_id %s: %w", sportradarID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetConference(conference.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested League pointer
	league, err := s.GetLeagueByID(ctx, int(conference.LeagueID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve league for conference %s: %w", sportradarID, err)
	}
	conference.League = league

	// Register and return
	return models.Registry.RegisterConference(&conference), nil
}
