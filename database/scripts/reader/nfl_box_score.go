package reader

import (
	"context"
	"fmt"

	nflmodels "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
)

// =============================================================================
// NFL Box Score Reader
// =============================================================================
// This package reads NFL box score data from the database for display and
// comparison purposes.
//
// Design principles:
// - Reads from database using store package
// - Returns shared/models types for use by other packages
// - Assembles complete NFLBoxScore with game and player information
// - No transformation logic - just data retrieval and assembly
// =============================================================================

// ReadNFLBoxScore reads complete box score data for a game from the database.
// Fetches game info (with team details) and all player box scores.
// The gameID is the database integer ID, not the vendor UUID.
// Returns all players in a flat list without roster validation.
func ReadNFLBoxScore(ctx context.Context, dbStore *store.Store, gameID int) (*nflmodels.NFLBoxScore, error) {
	// Step 1: Get game with team information
	game, err := dbStore.GetGameWithTeamsByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read game for game_id %d: %w", gameID, err)
	}

	// Step 2: Get all box scores for the game (with individual info)
	players, err := dbStore.GetNFLBoxScoresByGameID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read box scores for game_id %d: %w", gameID, err)
	}

	return &nflmodels.NFLBoxScore{
		Game:    game,
		Players: players,
	}, nil
}
