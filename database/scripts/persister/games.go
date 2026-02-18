package persister

import (
	"context"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/fetcher"
	"github.com/openbook/population-scripts/store"
)

// PersistNFLGames persists all NFL games from the schedule response.
// Returns the number of games upserted.
func PersistNFLGames(ctx context.Context, dbStore *store.Store, schedule *fetcher.NFLScheduleResponse) (int, error) {
	gameCount := 0

	for _, week := range schedule.Weeks {
		for _, gameData := range week.Games {
			if shouldExcludeGame(gameData.Home.ID, gameData.Away.ID, gameData.Home.Alias, gameData.Away.Alias) {
				continue
			}

			scheduledTime, err := time.Parse(time.RFC3339, gameData.Scheduled)
			if err != nil {
				return 0, fmt.Errorf("failed to parse scheduled time for game %s (%s %s vs %s %s): %w",
					gameData.ID, gameData.Home.Market, gameData.Home.Name, gameData.Away.Market, gameData.Away.Name, err)
			}

			homeTeam, err := dbStore.GetTeamBySportradarID(ctx, gameData.Home.ID)
			if err != nil {
				if shouldExcludeGameForTeamLookupErr(err) {
					continue
				}
				return 0, fmt.Errorf("failed to look up home team for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s): %w",
					gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
					gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
					gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias, err)
			}

			awayTeam, err := dbStore.GetTeamBySportradarID(ctx, gameData.Away.ID)
			if err != nil {
				if shouldExcludeGameForTeamLookupErr(err) {
					continue
				}
				return 0, fmt.Errorf("failed to look up away team for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s): %w",
					gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
					gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
					gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias, err)
			}

			err = dbStore.UpsertGame(ctx, &store.GameForUpsert{
				SportradarID:           gameData.ID,
				HomeTeamID:         homeTeam.ID,
				AwayTeamID:         awayTeam.ID,
				ScheduledStartTime: scheduledTime,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to upsert game %s: %w", gameData.ID, err)
			}

			gameCount++
		}
	}

	return gameCount, nil
}

// PersistNBAGames persists all NBA games from the schedule response.
// Returns the number of games upserted.
func PersistNBAGames(ctx context.Context, dbStore *store.Store, schedule *fetcher.NBAScheduleResponse) (int, error) {
	gameCount := 0

	for _, gameData := range schedule.Games {
		if shouldExcludeGame(gameData.Home.ID, gameData.Away.ID, gameData.Home.Alias, gameData.Away.Alias) {
			continue
		}

		scheduledTime, err := time.Parse(time.RFC3339, gameData.Scheduled)
		if err != nil {
			return 0, fmt.Errorf("failed to parse scheduled time for game %s (%s %s vs %s %s): %w",
				gameData.ID, gameData.Home.Market, gameData.Home.Name, gameData.Away.Market, gameData.Away.Name, err)
		}

		homeTeam, err := dbStore.GetTeamBySportradarID(ctx, gameData.Home.ID)
		if err != nil {
			if shouldExcludeGameForTeamLookupErr(err) {
				continue
			}
			return 0, fmt.Errorf("failed to look up home team for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s): %w",
				gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
				gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
				gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias, err)
		}

		awayTeam, err := dbStore.GetTeamBySportradarID(ctx, gameData.Away.ID)
		if err != nil {
			if shouldExcludeGameForTeamLookupErr(err) {
				continue
			}
			return 0, fmt.Errorf("failed to look up away team for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s): %w",
				gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
				gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
				gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias, err)
		}

		err = dbStore.UpsertGame(ctx, &store.GameForUpsert{
			SportradarID:           gameData.ID,
			HomeTeamID:         homeTeam.ID,
			AwayTeamID:         awayTeam.ID,
			ScheduledStartTime: scheduledTime,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to upsert game %s: %w", gameData.ID, err)
		}

		gameCount++
	}

	return gameCount, nil
}
