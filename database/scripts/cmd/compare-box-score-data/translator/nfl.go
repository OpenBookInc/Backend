package translator

import (
	"context"
	"fmt"

	"github.com/openbook/population-scripts/cmd/compare-box-score-data/fetcher"
	"github.com/openbook/population-scripts/store"
	models "github.com/openbook/shared/models"
	models_nfl "github.com/openbook/shared/models/nfl"
	"github.com/shopspring/decimal"
)

// nflPlayerStats holds aggregated statistics for a single NFL player
type nflPlayerStats struct {
	PassingCompletions  int
	PassingAttempts     int
	PassingYards        int
	PassingTouchdowns   int
	InterceptionsThrown int
	SacksTaken          int
	RushingAttempts     int
	RushingYards        int
	RushingTouchdowns   int
	ReceivingReceptions int
	ReceivingTargets    int
	ReceivingYards      int
	ReceivingTouchdowns int
	SacksMade           float64
	InterceptionsCaught int
	FumblesCommitted    int
	FieldGoalAttempts   int
	FieldGoalMakes      int
	ExtraPointAttempts  int
	ExtraPointMakes     int
}

// TranslateNFLBoxScore translates Sportradar NFL game statistics to an NFLBoxScore model.
// It aggregates stats from both home and away teams across all stat categories.
func TranslateNFLBoxScore(ctx context.Context, game *models.Game, stats *fetcher.NFLGameStatistics, dbStore *store.Store) (*models_nfl.NFLBoxScore, error) {
	// Aggregate stats by player vendor ID from both teams
	playerStats := make(map[string]*nflPlayerStats)

	// Process home team
	aggregateNFLTeamStats(&stats.Statistics.Home, playerStats)

	// Process away team
	aggregateNFLTeamStats(&stats.Statistics.Away, playerStats)

	// Build box score with individual lookups
	var players []*models_nfl.IndividualBoxScore

	for vendorID, pStats := range playerStats {
		individual, err := dbStore.GetIndividualByVendorID(ctx, vendorID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up individual with vendor ID %s: %w", vendorID, err)
		}

		nflStats := &models_nfl.NFLStats{
			Game:                game,
			Individual:          individual,
			PassingCompletions:  decimal.NewFromInt(int64(pStats.PassingCompletions)),
			PassingAttempts:     decimal.NewFromInt(int64(pStats.PassingAttempts)),
			PassingYards:        decimal.NewFromInt(int64(pStats.PassingYards)),
			PassingTouchdowns:   decimal.NewFromInt(int64(pStats.PassingTouchdowns)),
			InterceptionsThrown: decimal.NewFromInt(int64(pStats.InterceptionsThrown)),
			SacksTaken:          decimal.NewFromInt(int64(pStats.SacksTaken)),
			RushingAttempts:     decimal.NewFromInt(int64(pStats.RushingAttempts)),
			RushingYards:        decimal.NewFromInt(int64(pStats.RushingYards)),
			RushingTouchdowns:   decimal.NewFromInt(int64(pStats.RushingTouchdowns)),
			ReceivingReceptions: decimal.NewFromInt(int64(pStats.ReceivingReceptions)),
			ReceivingTargets:    decimal.NewFromInt(int64(pStats.ReceivingTargets)),
			ReceivingYards:      decimal.NewFromInt(int64(pStats.ReceivingYards)),
			ReceivingTouchdowns: decimal.NewFromInt(int64(pStats.ReceivingTouchdowns)),
			SacksMade:           decimal.NewFromFloat(pStats.SacksMade),
			SackAssistsMade:     decimal.Zero, // Sportradar doesn't separate sack assists
			InterceptionsCaught: decimal.NewFromInt(int64(pStats.InterceptionsCaught)),
			FumblesCommitted:    decimal.NewFromInt(int64(pStats.FumblesCommitted)),
			FieldGoalAttempts:   decimal.NewFromInt(int64(pStats.FieldGoalAttempts)),
			FieldGoalMakes:      decimal.NewFromInt(int64(pStats.FieldGoalMakes)),
			ExtraPointAttempts:  decimal.NewFromInt(int64(pStats.ExtraPointAttempts)),
			ExtraPointMakes:     decimal.NewFromInt(int64(pStats.ExtraPointMakes)),
		}

		players = append(players, &models_nfl.IndividualBoxScore{
			Individual: individual,
			Stats:      nflStats,
		})
	}

	return &models_nfl.NFLBoxScore{
		Game:    game,
		Players: players,
	}, nil
}

// aggregateNFLTeamStats adds stats from a single team to the player stats map
func aggregateNFLTeamStats(team *fetcher.NFLTeamStatistics, playerStats map[string]*nflPlayerStats) {
	// Process passing stats
	for _, p := range team.Passing.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.PassingCompletions += p.Completions
		stats.PassingAttempts += p.Attempts
		stats.PassingYards += p.Yards
		stats.PassingTouchdowns += p.Touchdowns
		stats.InterceptionsThrown += p.Interceptions
		stats.SacksTaken += p.Sacks
	}

	// Process rushing stats
	for _, p := range team.Rushing.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.RushingAttempts += p.Attempts
		stats.RushingYards += p.Yards
		stats.RushingTouchdowns += p.Touchdowns
	}

	// Process receiving stats
	for _, p := range team.Receiving.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.ReceivingReceptions += p.Receptions
		stats.ReceivingTargets += p.Targets
		stats.ReceivingYards += p.Yards
		stats.ReceivingTouchdowns += p.Touchdowns
	}

	// Process defense stats
	for _, p := range team.Defense.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.SacksMade += p.Sacks
		stats.InterceptionsCaught += p.Interceptions
	}

	// Process fumbles stats
	for _, p := range team.Fumbles.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.FumblesCommitted += p.Fumbles
	}

	// Process field goal stats
	for _, p := range team.FieldGoals.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.FieldGoalAttempts += p.Attempts
		stats.FieldGoalMakes += p.Made
	}

	// Process extra point kicks stats
	for _, p := range team.ExtraPoints.Kicks.Players {
		stats := getOrCreatePlayerStats(playerStats, p.ID)
		stats.ExtraPointAttempts += p.Attempts
		stats.ExtraPointMakes += p.Made
	}
}

// getOrCreatePlayerStats returns existing stats for a player or creates new ones
func getOrCreatePlayerStats(playerStats map[string]*nflPlayerStats, vendorID string) *nflPlayerStats {
	if stats, exists := playerStats[vendorID]; exists {
		return stats
	}
	stats := &nflPlayerStats{}
	playerStats[vendorID] = stats
	return stats
}
