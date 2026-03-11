package nfl

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/store"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/shared/utils"
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
// The gameID is the database UUID, not the vendor UUID.
// Returns all players in a flat list without roster validation.
func ReadNFLBoxScore(ctx context.Context, dbStore *store.Store, gameID utils.UUID) (*models_nfl.NFLBoxScore, error) {
	// Step 1: Get game with team information
	game, err := dbStore.GetGameWithTeamsByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read game for game_id %s: %w", gameID, err)
	}

	// Step 2: Get all box scores for the game (with individual info)
	players, err := store_nfl.GetNFLBoxScoresByGameID(dbStore, ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read box scores for game_id %s: %w", gameID, err)
	}

	return &models_nfl.NFLBoxScore{
		Game:    game,
		Players: players,
	}, nil
}
