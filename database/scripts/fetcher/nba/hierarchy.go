package nba

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NBAHierarchyResponse represents the NBA league hierarchy API response
type NBAHierarchyResponse struct {
	Conferences []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Alias     string `json:"alias"`
		Divisions []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Alias string `json:"alias"`
			Teams []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Market string `json:"market"`
				Alias  string `json:"alias"`
				Venue  struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					City     string `json:"city"`
					State    string `json:"state"`
					Capacity int    `json:"capacity"`
				} `json:"venue"`
			} `json:"teams"`
		} `json:"divisions"`
	} `json:"conferences"`
}

// FetchNBAHierarchy fetches the NBA league hierarchy from the Sportradar API
func FetchNBAHierarchy(apiClient *sportradar.Client) (*NBAHierarchyResponse, error) {
	teamsData, err := apiClient.GetNBATeams()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA teams: %w", err)
	}

	var hierarchyResp NBAHierarchyResponse
	if err := json.Unmarshal(teamsData, &hierarchyResp); err != nil {
		return nil, fmt.Errorf("failed to parse NBA teams response: %w", err)
	}

	return &hierarchyResp, nil
}
