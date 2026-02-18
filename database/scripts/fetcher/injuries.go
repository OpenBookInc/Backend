package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NFLInjuriesResponse represents the NFL weekly injuries API response
type NFLInjuriesResponse struct {
	Teams []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Market  string `json:"market"`
		Alias   string `json:"alias"`
		Players []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Position string `json:"position"`
			Jersey   string `json:"jersey"`
			Injuries []struct {
				Status  string `json:"status"`  // e.g., "Out", "Questionable", "Doubtful" - game status
				Primary string `json:"primary"` // Injury description (e.g., "Hip", "Shoulder")
			} `json:"injuries"`
		} `json:"players"`
	} `json:"teams"`
}

// NBAInjuriesResponse represents the NBA injuries API response
type NBAInjuriesResponse struct {
	Teams []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Market  string `json:"market"`
		Players []struct {
			ID        string `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			FullName  string `json:"full_name"`
			Position  string `json:"position"`
			Injuries  []struct {
				Status  string `json:"status"`  // e.g., "Out", "Day To Day"
				Desc    string `json:"desc"`    // Injury description (e.g., "Hamstring", "Calf")
				Comment string `json:"comment"` // Additional details
			} `json:"injuries"`
		} `json:"players"`
	} `json:"teams"`
}

// FetchNFLPlayerStatuses fetches player injury statuses from the NFL weekly injuries endpoint
func FetchNFLPlayerStatuses(apiClient *sportradar.Client, year int, seasonType string, week int) (*NFLInjuriesResponse, error) {
	injuriesData, err := apiClient.GetNFLWeeklyInjuries(year, seasonType, week)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL weekly injuries: %w", err)
	}

	var injuriesResp NFLInjuriesResponse
	if err := json.Unmarshal(injuriesData, &injuriesResp); err != nil {
		return nil, fmt.Errorf("failed to parse NFL injuries response: %w", err)
	}

	return &injuriesResp, nil
}

// FetchNBAPlayerStatuses fetches player injury statuses from the NBA injuries endpoint
func FetchNBAPlayerStatuses(apiClient *sportradar.Client) (*NBAInjuriesResponse, error) {
	injuriesData, err := apiClient.GetNBAInjuries()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA injuries: %w", err)
	}

	var injuriesResp NBAInjuriesResponse
	if err := json.Unmarshal(injuriesData, &injuriesResp); err != nil {
		return nil, fmt.Errorf("failed to parse NBA injuries response: %w", err)
	}

	return &injuriesResp, nil
}
