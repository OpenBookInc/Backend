package nfl

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NFLHierarchyResponse represents the NFL league hierarchy API response
type NFLHierarchyResponse struct {
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

// FetchNFLHierarchy fetches the NFL league hierarchy from the Sportradar API
func FetchNFLHierarchy(apiClient *sportradar.Client) (*NFLHierarchyResponse, error) {
	teamsData, err := apiClient.GetNFLTeams()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL teams: %w", err)
	}

	var hierarchyResp NFLHierarchyResponse
	if err := json.Unmarshal(teamsData, &hierarchyResp); err != nil {
		return nil, fmt.Errorf("failed to parse NFL teams response: %w", err)
	}

	return &hierarchyResp, nil
}
