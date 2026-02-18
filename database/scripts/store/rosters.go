package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
)

// RosterForUpsert contains the data needed to upsert a roster
type RosterForUpsert struct {
	TeamID        int
	IndividualIDs []int64
}

// UpsertRoster inserts or updates a roster in the database.
// Uses team_id as the unique identifier (ON CONFLICT).
// Only stores the latest roster for each team (no historical tracking).
// Resolves the Team pointer and registers in the singleton registry.
func (s *Store) UpsertRoster(ctx context.Context, roster *RosterForUpsert) error {
	query := `
		INSERT INTO rosters (team_id, individual_ids)
		VALUES ($1, $2)
		ON CONFLICT (team_id)
		DO UPDATE SET individual_ids = EXCLUDED.individual_ids
		RETURNING id
	`

	var id int
	err := s.pool.QueryRow(ctx, query, roster.TeamID, roster.IndividualIDs).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to upsert roster for team_id %d: %w", roster.TeamID, err)
	}

	team, err := s.GetTeamByID(ctx, roster.TeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve team for roster (team_id %d): %w", roster.TeamID, err)
	}

	models.Registry.RegisterRoster(&models.Roster{
		ID:            id,
		TeamID:        int64(roster.TeamID),
		IndividualIDs: roster.IndividualIDs,
		Team:          team,
	})
	return nil
}

// GetRosterByID retrieves a roster by database ID.
// Uses the registry for caching and resolves nested Team and Players pointers.
func (s *Store) GetRosterByID(ctx context.Context, id int) (*models.Roster, error) {
	// Check registry first
	if roster := models.Registry.GetRoster(id); roster != nil {
		return roster, nil
	}

	// Query database
	query := `SELECT id, team_id, individual_ids FROM rosters WHERE id = $1`

	var roster models.Roster
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&roster.ID,
		&roster.TeamID,
		&roster.IndividualIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get roster with id %d: %w", id, err)
	}

	// Resolve nested Team pointer
	team, err := s.GetTeamByID(ctx, int(roster.TeamID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team for roster %d: %w", id, err)
	}
	roster.Team = team

	// Resolve nested Players pointers
	roster.Players = make([]*models.Individual, 0, len(roster.IndividualIDs))
	for _, individualID := range roster.IndividualIDs {
		individual, err := s.GetIndividualByID(ctx, int(individualID))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve individual %d for roster %d: %w", individualID, id, err)
		}
		roster.Players = append(roster.Players, individual)
	}

	// Register and return
	return models.Registry.RegisterRoster(&roster), nil
}

// GetRosterByTeamID retrieves a roster by team_id.
// Uses the registry for caching and resolves nested Team and Players pointers.
func (s *Store) GetRosterByTeamID(ctx context.Context, teamID int64) (*models.Roster, error) {
	// Query database to get the ID first
	query := `SELECT id, team_id, individual_ids FROM rosters WHERE team_id = $1`

	var roster models.Roster
	err := s.pool.QueryRow(ctx, query, teamID).Scan(
		&roster.ID,
		&roster.TeamID,
		&roster.IndividualIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get roster for team_id %d: %w", teamID, err)
	}

	// Check if already registered (by ID)
	if existing := models.Registry.GetRoster(roster.ID); existing != nil {
		return existing, nil
	}

	// Resolve nested Team pointer
	team, err := s.GetTeamByID(ctx, int(roster.TeamID))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team for roster (team_id %d): %w", teamID, err)
	}
	roster.Team = team

	// Resolve nested Players pointers
	roster.Players = make([]*models.Individual, 0, len(roster.IndividualIDs))
	for _, individualID := range roster.IndividualIDs {
		individual, err := s.GetIndividualByID(ctx, int(individualID))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve individual %d for roster (team_id %d): %w", individualID, teamID, err)
		}
		roster.Players = append(roster.Players, individual)
	}

	// Register and return
	return models.Registry.RegisterRoster(&roster), nil
}
