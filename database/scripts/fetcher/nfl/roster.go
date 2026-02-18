package nfl

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NFLTeamRosterResponse represents the NFL team roster API response
type NFLTeamRosterResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Market  string `json:"market"`
	Alias   string `json:"alias"`
	Players []struct {
		ID          string      `json:"id"`
		Name        string      `json:"name"`
		FirstName   string      `json:"first_name"`
		LastName    string      `json:"last_name"`
		NameSuffix  string      `json:"name_suffix"`
		Position    string      `json:"position"`
		JerseyNum   string      `json:"jersey"`
		Height      interface{} `json:"height"` // Can be int or float
		Weight      interface{} `json:"weight"` // Can be int or float
		BirthDate   string      `json:"birth_date"`
		Status      string      `json:"status"`
	} `json:"players"`
}

// FetchNFLTeamRoster fetches roster data for a single NFL team
func FetchNFLTeamRoster(apiClient *sportradar.Client, teamSportradarID string) (*NFLTeamRosterResponse, error) {
	rosterData, err := apiClient.GetNFLTeamRoster(teamSportradarID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL team roster for %s: %w", teamSportradarID, err)
	}

	var rosterResp NFLTeamRosterResponse
	if err := json.Unmarshal(rosterData, &rosterResp); err != nil {
		return nil, fmt.Errorf("failed to parse team roster response: %w", err)
	}

	return &rosterResp, nil
}
