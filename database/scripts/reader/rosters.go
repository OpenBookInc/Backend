package reader

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
)

// =============================================================================
// Roster Reader
// =============================================================================
// This package reads roster data from the database for use in organizing
// box score data by team membership.
//
// Design principles:
// - Reads from database using store package
// - Returns shared/models types for use by other packages
// - No transformation logic - just data retrieval
// =============================================================================

// ReadRosterByTeamID reads a roster from the database by team ID.
// Returns the roster with individual IDs populated.
// The teamID is the database UUID (ContenderIDA or ContenderIDB from Game).
func ReadRosterByTeamID(ctx context.Context, dbStore *store.Store, teamID string) (*models.Roster, error) {
	roster, err := dbStore.GetRosterByTeamID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to read roster for team_id %s: %w", teamID, err)
	}

	return roster, nil
}
