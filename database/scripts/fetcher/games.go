package fetcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NFLScheduleResponse represents the NFL season schedule API response
type NFLScheduleResponse struct {
	Weeks []struct {
		ID     string `json:"id"`
		Number int    `json:"number"`
		Games  []struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			Scheduled string `json:"scheduled"`
			Home      struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Market string `json:"market"`
				Alias  string `json:"alias"`
			} `json:"home"`
			Away struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Market string `json:"market"`
				Alias  string `json:"alias"`
			} `json:"away"`
		} `json:"games"`
	} `json:"weeks"`
}

// NBAScheduleResponse represents the NBA season schedule API response
type NBAScheduleResponse struct {
	Games []struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Scheduled string `json:"scheduled"`
		Home      struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Market string `json:"market"`
			Alias  string `json:"alias"`
		} `json:"home"`
		Away struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Market string `json:"market"`
			Alias  string `json:"alias"`
		} `json:"away"`
	} `json:"games"`
}

// FetchNFLGames fetches all games from the NFL season schedule
func FetchNFLGames(apiClient *sportradar.Client, dataStore *ReferenceData, year int, seasonType string) error {
	scheduleData, err := apiClient.GetNFLSeasonSchedule(year, seasonType)
	if err != nil {
		return fmt.Errorf("failed to fetch NFL season schedule: %w", err)
	}

	var scheduleResp NFLScheduleResponse
	if err := json.Unmarshal(scheduleData, &scheduleResp); err != nil {
		return fmt.Errorf("failed to parse NFL schedule response: %w", err)
	}

	// Process all weeks and games
	for _, week := range scheduleResp.Weeks {
		for _, gameData := range week.Games {
			// Check exclusion rules first
			if shouldExcludeGame(gameData.Home.ID, gameData.Away.ID, gameData.Home.Alias, gameData.Away.Alias) {
				// Silently skip excluded games (e.g., TBD teams, undetermined playoff matchups)
				continue
			}

			// Parse scheduled time
			scheduledTime, err := time.Parse(time.RFC3339, gameData.Scheduled)
			if err != nil {
				return fmt.Errorf("failed to parse scheduled time for game %s (%s %s vs %s %s): %w",
					gameData.ID, gameData.Home.Market, gameData.Home.Name, gameData.Away.Market, gameData.Away.Name, err)
			}

			// Get teams from dataStore
			homeTeam := dataStore.Teams[gameData.Home.ID]
			awayTeam := dataStore.Teams[gameData.Away.ID]

			if homeTeam == nil {
				return fmt.Errorf("home team not found for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s)",
					gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
					gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
					gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias)
			}
			if awayTeam == nil {
				return fmt.Errorf("away team not found for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s)",
					gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
					gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
					gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias)
			}

			game := &Game{
				VendorID:           gameData.ID,
				ScheduledStartTime: scheduledTime,
				HomeTeam:           homeTeam,
				AwayTeam:           awayTeam,
			}

			dataStore.AddGame(game)
		}
	}

	return nil
}

// FetchNBAGames fetches all games from the NBA season schedule
func FetchNBAGames(apiClient *sportradar.Client, dataStore *ReferenceData, year int, seasonType string) error {
	scheduleData, err := apiClient.GetNBASeasonSchedule(year, seasonType)
	if err != nil {
		return fmt.Errorf("failed to fetch NBA season schedule: %w", err)
	}

	var scheduleResp NBAScheduleResponse
	if err := json.Unmarshal(scheduleData, &scheduleResp); err != nil {
		return fmt.Errorf("failed to parse NBA schedule response: %w", err)
	}

	// Process all games
	for _, gameData := range scheduleResp.Games {
		// Check exclusion rules first
		if shouldExcludeGame(gameData.Home.ID, gameData.Away.ID, gameData.Home.Alias, gameData.Away.Alias) {
			// Silently skip excluded games (e.g., TBD teams, undetermined playoff matchups)
			continue
		}

		// Parse scheduled time
		scheduledTime, err := time.Parse(time.RFC3339, gameData.Scheduled)
		if err != nil {
			return fmt.Errorf("failed to parse scheduled time for game %s (%s %s vs %s %s): %w",
				gameData.ID, gameData.Home.Market, gameData.Home.Name, gameData.Away.Market, gameData.Away.Name, err)
		}

		// Get teams from dataStore
		homeTeam := dataStore.Teams[gameData.Home.ID]
		awayTeam := dataStore.Teams[gameData.Away.ID]

		if homeTeam == nil {
			return fmt.Errorf("home team not found for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s)",
				gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
				gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
				gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias)
		}
		if awayTeam == nil {
			return fmt.Errorf("away team not found for game %s (scheduled: %s)\n  Home: %s %s (ID: %s, Alias: %s)\n  Away: %s %s (ID: %s, Alias: %s)",
				gameData.ID, scheduledTime.Format("2006-01-02 15:04 MST"),
				gameData.Home.Market, gameData.Home.Name, gameData.Home.ID, gameData.Home.Alias,
				gameData.Away.Market, gameData.Away.Name, gameData.Away.ID, gameData.Away.Alias)
		}

		game := &Game{
			VendorID:           gameData.ID,
			ScheduledStartTime: scheduledTime,
			HomeTeam:           homeTeam,
			AwayTeam:           awayTeam,
		}

		dataStore.AddGame(game)
	}

	return nil
}
