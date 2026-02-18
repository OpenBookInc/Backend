package nba

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
)

// NBATeamProfileResponse represents the NBA team profile API response
type NBATeamProfileResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Market  string `json:"market"`
	Alias   string `json:"alias"`
	Players []struct {
		ID          string      `json:"id"`
		FirstName   string      `json:"first_name"`
		LastName    string      `json:"last_name"`
		FullName    string      `json:"full_name"`
		Position    string      `json:"position"`
		Primary     string      `json:"primary_position"`
		JerseyNum   string      `json:"jersey_number"`
		Height      interface{} `json:"height"` // Can be string or number
		Weight      interface{} `json:"weight"` // Can be string or number
		BirthDate   string      `json:"birthdate"`
		Status      string      `json:"status"`
	} `json:"players"`
}

// FetchNBATeamRoster fetches roster data for a single NBA team
func FetchNBATeamRoster(apiClient *sportradar.Client, teamVendorID string) (*NBATeamProfileResponse, error) {
	rosterData, err := apiClient.GetNBATeamRoster(teamVendorID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA team roster for %s: %w", teamVendorID, err)
	}

	var profileResp NBATeamProfileResponse
	if err := json.Unmarshal(rosterData, &profileResp); err != nil {
		return nil, fmt.Errorf("failed to parse team roster response: %w", err)
	}

	return &profileResp, nil
}
