package fetcher

import (
	"encoding/json"
	"fmt"

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

// FetchNFLGames fetches the NFL season schedule from the Sportradar API
func FetchNFLGames(apiClient *sportradar.Client, year int, seasonType string) (*NFLScheduleResponse, error) {
	scheduleData, err := apiClient.GetNFLSeasonSchedule(year, seasonType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL season schedule: %w", err)
	}

	var scheduleResp NFLScheduleResponse
	if err := json.Unmarshal(scheduleData, &scheduleResp); err != nil {
		return nil, fmt.Errorf("failed to parse NFL schedule response: %w", err)
	}

	return &scheduleResp, nil
}

// FetchNBAGames fetches the NBA season schedule from the Sportradar API
func FetchNBAGames(apiClient *sportradar.Client, year int, seasonType string) (*NBAScheduleResponse, error) {
	scheduleData, err := apiClient.GetNBASeasonSchedule(year, seasonType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA season schedule: %w", err)
	}

	var scheduleResp NBAScheduleResponse
	if err := json.Unmarshal(scheduleData, &scheduleResp); err != nil {
		return nil, fmt.Errorf("failed to parse NBA schedule response: %w", err)
	}

	return &scheduleResp, nil
}
