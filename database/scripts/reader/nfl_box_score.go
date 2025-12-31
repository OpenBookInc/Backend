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
// Players are organized into HomeTeamPlayers (ContenderIDA) and AwayTeamPlayers (ContenderIDB).
func ReadNFLBoxScore(ctx context.Context, dbStore *store.Store, gameID int) (*nflmodels.NFLBoxScore, error) {
	// Step 1: Get game with team information
	game, err := dbStore.GetGameWithTeamsByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read game for game_id %d: %w", gameID, err)
	}

	// Step 2: Get all box scores for the game (with individual info and team_id)
	players, err := dbStore.GetNFLBoxScoresByGameID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to read box scores for game_id %d: %w", gameID, err)
	}

	// Step 3: Split players into home and away teams
	// ContenderIDA/B are int64, TeamID is int - cast for comparison
	var homeTeamPlayers []*nflmodels.IndividualBoxScore
	var awayTeamPlayers []*nflmodels.IndividualBoxScore

	for _, player := range players {
		teamID := int64(player.TeamID)
		switch teamID {
		case game.ContenderIDA:
			// ContenderIDA is the home team
			homeTeamPlayers = append(homeTeamPlayers, player)
		case game.ContenderIDB:
			// ContenderIDB is the away team
			awayTeamPlayers = append(awayTeamPlayers, player)
		default:
			// Player's team doesn't match either contender (roster issue or free agent)
			// Skip silently - this can happen if roster data is stale
		}
	}

	return &nflmodels.NFLBoxScore{
		Game:            game,
		HomeTeamPlayers: homeTeamPlayers,
		AwayTeamPlayers: awayTeamPlayers,
	}, nil
}
