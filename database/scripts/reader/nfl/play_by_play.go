package nfl

import (
	"context"
	"fmt"

	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/openbook/population-scripts/store"
	store_nfl "github.com/openbook/population-scripts/store/nfl"
)

// =============================================================================
// NFL Play-by-Play Reader
// =============================================================================
// This package reads NFL play-by-play data from the database for use in
// downstream processing (e.g., box score generation).
//
// Design principles:
// - Reads from database using store package
// - Returns shared/models types for use by other packages
// - No transformation logic - just data retrieval
// =============================================================================

// NFLPlayByPlayData holds all play statistics for a game
type NFLPlayByPlayData struct {
	GameID     string
	Statistics []*models_nfl.PlayStatistic
}

// ReadNFLPlayByPlay reads all play statistics for a game from the database.
// Returns all PlayStatistic records associated with the given game_id.
// The game_id is the database UUID, not the vendor UUID.
func ReadNFLPlayByPlay(ctx context.Context, dbStore *store.Store, gameID string) (*NFLPlayByPlayData, error) {
	stats, err := store_nfl.GetNFLPlayStatisticsByGameID(dbStore, ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read play-by-play statistics for game_id %s: %w", gameID, err)
	}

	return &NFLPlayByPlayData{
		GameID:     gameID,
		Statistics: stats,
	}, nil
}
