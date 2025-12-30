package store

import (
	"context"
	"fmt"

	"github.com/openbook/shared/models"
)

// RosterForUpsert contains the data needed to upsert a roster
type RosterForUpsert struct {
	TeamID        int
	IndividualIDs []int64
}

// UpsertRoster inserts or updates a roster in the database
// Uses team_id as the unique identifier (ON CONFLICT)
// Only stores the latest roster for each team (no historical tracking)
// Returns the database ID of the roster
func (s *Store) UpsertRoster(ctx context.Context, roster *RosterForUpsert) (int, error) {
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
		return 0, fmt.Errorf("failed to upsert roster for team_id %d: %w", roster.TeamID, err)
	}

	return id, nil
}

// GetRosterByTeamID retrieves a roster by team_id
func (s *Store) GetRosterByTeamID(ctx context.Context, teamID int64) (*models.Roster, error) {
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

	return &roster, nil
}
