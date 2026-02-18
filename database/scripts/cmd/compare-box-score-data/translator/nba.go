package translator

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/cmd/compare-box-score-data/fetcher"
	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
	models_nba "github.com/openbook/shared/models/nba"
	"github.com/shopspring/decimal"
)

// TranslateNBABoxScore translates Sportradar NBA game summary to an NBABoxScore model.
// It processes players from both home and away teams.
func TranslateNBABoxScore(ctx context.Context, game *models.Game, summary *fetcher.NBAGameSummary, dbStore *store.Store) (*models_nba.NBABoxScore, error) {
	var players []*models_nba.IndividualBoxScore

	// Process home team players
	homePlayers, err := translateNBATeamPlayers(ctx, game, summary.Home.Players, dbStore)
	if err != nil {
		return nil, fmt.Errorf("failed to translate home team players: %w", err)
	}
	players = append(players, homePlayers...)

	// Process away team players
	awayPlayers, err := translateNBATeamPlayers(ctx, game, summary.Away.Players, dbStore)
	if err != nil {
		return nil, fmt.Errorf("failed to translate away team players: %w", err)
	}
	players = append(players, awayPlayers...)

	return &models_nba.NBABoxScore{
		Game:    game,
		Players: players,
	}, nil
}

// translateNBATeamPlayers translates players from a single team
func translateNBATeamPlayers(ctx context.Context, game *models.Game, players []fetcher.NBAPlayer, dbStore *store.Store) ([]*models_nba.IndividualBoxScore, error) {
	var result []*models_nba.IndividualBoxScore

	for _, p := range players {
		// Skip players with no stats (DNP)
		if !hasNBAStats(&p.Statistics) {
			continue
		}

		individual, err := dbStore.GetIndividualBySportradarID(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up individual %s (%s): %w", p.FullName, p.ID, err)
		}

		stats := &models_nba.NBAStats{
			Game:                   game,
			Individual:             individual,
			TwoPointAttempts:       decimal.NewFromInt(int64(p.Statistics.TwoPointsAtt)),
			TwoPointMakes:          decimal.NewFromInt(int64(p.Statistics.TwoPointsMade)),
			ThreePointAttempts:     decimal.NewFromInt(int64(p.Statistics.ThreePointsAtt)),
			ThreePointMakes:        decimal.NewFromInt(int64(p.Statistics.ThreePointsMade)),
			FreeThrowAttempts:      decimal.NewFromInt(int64(p.Statistics.FreeThrowsAtt)),
			FreeThrowMakes:         decimal.NewFromInt(int64(p.Statistics.FreeThrowsMade)),
			Assists:                decimal.NewFromInt(int64(p.Statistics.Assists)),
			DefensiveRebounds:      decimal.NewFromInt(int64(p.Statistics.DefensiveRebounds)),
			OffensiveRebounds:      decimal.NewFromInt(int64(p.Statistics.OffensiveRebounds)),
			Steals:                 decimal.NewFromInt(int64(p.Statistics.Steals)),
			Blocks:                 decimal.NewFromInt(int64(p.Statistics.Blocks)),
			TurnoversCommitted:     decimal.NewFromInt(int64(p.Statistics.Turnovers)),
			PersonalFoulsCommitted: decimal.NewFromInt(int64(p.Statistics.PersonalFouls)),
		}

		result = append(result, &models_nba.IndividualBoxScore{
			Individual: individual,
			Stats:      stats,
		})
	}

	return result, nil
}

// hasNBAStats returns true if the player has any recorded statistics
func hasNBAStats(stats *fetcher.NBAPlayerStatistics) bool {
	return stats.TwoPointsAtt > 0 ||
		stats.TwoPointsMade > 0 ||
		stats.ThreePointsAtt > 0 ||
		stats.ThreePointsMade > 0 ||
		stats.FreeThrowsAtt > 0 ||
		stats.FreeThrowsMade > 0 ||
		stats.Assists > 0 ||
		stats.DefensiveRebounds > 0 ||
		stats.OffensiveRebounds > 0 ||
		stats.Steals > 0 ||
		stats.Blocks > 0 ||
		stats.Turnovers > 0 ||
		stats.PersonalFouls > 0
}
