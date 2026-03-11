package nba

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
	models_nba "github.com/openbook/shared/models/nba"
	"github.com/openbook/shared/utils"
)

// =============================================================================
// NBA Box Score Reader
// =============================================================================
// This package reads NBA box score data from the database for display and
// comparison purposes.
//
// Design principles:
// - Reads from database using store package
// - Returns shared/models types for use by other packages
// - Assembles complete NBABoxScore with game and player information
// - No transformation logic - just data retrieval and assembly
// =============================================================================

// ReadNBABoxScore reads complete box score data for a game from the database.
// Fetches game info (with team details) and all player box scores.
// The gameID is the database UUID, not the vendor UUID.
// Returns all players in a flat list without roster validation.
func ReadNBABoxScore(ctx context.Context, dbStore *store.Store, gameID utils.UUID) (*models_nba.NBABoxScore, error) {
	// Step 1: Get game with team information
	game, err := dbStore.GetGameWithTeamsByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read game for game_id %s: %w", gameID, err)
	}

	// Step 2: Get all box scores for the game (with individual info)
	players, err := store_nba.GetNBABoxScoresByGameID(dbStore, ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read box scores for game_id %s: %w", gameID, err)
	}

	return &models_nba.NBABoxScore{
		Game:    game,
		Players: players,
	}, nil
}
