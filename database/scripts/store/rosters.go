package store

import (
	"context"
	"fmt"

	models "github.com/openbook/shared/models"
	"github.com/openbook/shared/utils"
)

// RosterForUpsert contains the data needed to upsert a roster
type RosterForUpsert struct {
	TeamID        utils.UUID
	IndividualIDs []utils.UUID
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
	`

	_, err := s.pool.Exec(ctx, query, roster.TeamID, roster.IndividualIDs)
	if err != nil {
		return fmt.Errorf("failed to upsert roster for team_id %s: %w", roster.TeamID, err)
	}

	team, err := s.GetTeamByID(ctx, roster.TeamID)
	if err != nil {
		return fmt.Errorf("failed to resolve team for roster (team_id %s): %w", roster.TeamID, err)
	}

	models.Registry.RegisterRoster(&models.Roster{
		TeamID:        roster.TeamID,
		IndividualIDs: roster.IndividualIDs,
		Team:          team,
	})
	return nil
}

// GetRostersByLeague retrieves all rosters for a given league name in a single query.
// JOINs through teams -> divisions -> conferences -> leagues to filter by league.
// Resolves Team pointers from registry (teams should already be loaded).
// Does NOT resolve Players pointers - caller can use IndividualIDs directly.
func (s *Store) GetRostersByLeague(ctx context.Context, leagueName string) ([]*models.Roster, error) {
	query := `
		SELECT r.team_id, r.individual_ids
		FROM rosters r
		JOIN teams t ON r.team_id = t.id
		JOIN divisions d ON t.division_id = d.id
		JOIN conferences c ON d.conference_id = c.id
		JOIN leagues l ON c.league_id = l.id
		WHERE l.name = $1
		ORDER BY r.team_id ASC
	`

	rows, err := s.pool.Query(ctx, query, leagueName)
	if err != nil {
		return nil, fmt.Errorf("failed to query rosters for league %s: %w", leagueName, err)
	}
	defer rows.Close()

	var rosters []*models.Roster
	for rows.Next() {
		var roster models.Roster
		if err := rows.Scan(&roster.TeamID, &roster.IndividualIDs); err != nil {
			return nil, fmt.Errorf("failed to scan roster row: %w", err)
		}
		roster.Team = models.Registry.GetTeam(roster.TeamID)
		rosters = append(rosters, models.Registry.RegisterRoster(&roster))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating roster rows: %w", err)
	}

	return rosters, nil
}

// GetRosterByTeamID retrieves a roster by team_id.
// Uses the registry for caching and resolves nested Team and Players pointers.
func (s *Store) GetRosterByTeamID(ctx context.Context, teamID utils.UUID) (*models.Roster, error) {
	// Check registry first
	if roster := models.Registry.GetRoster(teamID); roster != nil {
		return roster, nil
	}

	// Query database
	query := `SELECT team_id, individual_ids FROM rosters WHERE team_id = $1`

	var roster models.Roster
	err := s.pool.QueryRow(ctx, query, teamID).Scan(
		&roster.TeamID,
		&roster.IndividualIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get roster for team_id %s: %w", teamID, err)
	}

	// Resolve nested Team pointer
	team, err := s.GetTeamByID(ctx, roster.TeamID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve team for roster (team_id %s): %w", teamID, err)
	}
	roster.Team = team

	// Resolve nested Players pointers
	roster.Players = make([]*models.Individual, 0, len(roster.IndividualIDs))
	for _, individualID := range roster.IndividualIDs {
		individual, err := s.GetIndividualByID(ctx, individualID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve individual %s for roster (team_id %s): %w", individualID, teamID, err)
		}
		roster.Players = append(roster.Players, individual)
	}

	// Register and return
	return models.Registry.RegisterRoster(&roster), nil
}
