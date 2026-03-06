package nba

import (
	"context"
	"fmt"

	models_nba "github.com/openbook/shared/models/nba"
	"github.com/openbook/population-scripts/store"
	store_nba "github.com/openbook/population-scripts/store/nba"
)

// =============================================================================
// NBA Play-by-Play Reader
// =============================================================================
// This package reads NBA play-by-play data from the database for use in
// downstream processing (e.g., box score generation).
//
// Design principles:
// - Reads from database using store package
// - Returns shared/models types for use by other packages
// - No transformation logic - just data retrieval
// =============================================================================

// NBAPlayByPlayData holds all play statistics for a game
type NBAPlayByPlayData struct {
	GameID     string
	Statistics []*models_nba.PlayStatistic
}

// ReadNBAPlayByPlay reads all play statistics for a game from the database.
// Returns all PlayStatistic records associated with the given game_id.
// The game_id is the database UUID, not the vendor UUID.
func ReadNBAPlayByPlay(ctx context.Context, dbStore *store.Store, gameID string) (*NBAPlayByPlayData, error) {
	stats, err := store_nba.GetNBAPlayStatisticsByGameID(dbStore, ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read play-by-play statistics for game_id %s: %w", gameID, err)
	}

	return &NBAPlayByPlayData{
		GameID:     gameID,
		Statistics: stats,
	}, nil
}
