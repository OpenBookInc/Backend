package nfl

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/fetcher"
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
func FetchNFLTeamRoster(apiClient *sportradar.Client, dataStore *fetcher.ReferenceData, teamVendorID string, league *fetcher.League) error {
	rosterData, err := apiClient.GetNFLTeamRoster(teamVendorID)
	if err != nil {
		return err
	}

	var rosterResp NFLTeamRosterResponse
	if err := json.Unmarshal(rosterData, &rosterResp); err != nil {
		return fmt.Errorf("failed to parse team roster response: %w", err)
	}

	// Get the team from dataStore to link roster
	team := dataStore.Teams[teamVendorID]

	roster := &fetcher.Roster{
		Team:    team,
		Players: []*fetcher.Individual{},
	}

	for _, playerData := range rosterResp.Players {
		// Parse birth date
		var dateOfBirth *time.Time
		if playerData.BirthDate != "" {
			if parsedDate, err := time.Parse("2006-01-02", playerData.BirthDate); err == nil {
				dateOfBirth = &parsedDate
			}
		}

		// Create abbreviated name (e.g., "J.Smith" from "John Smith")
		abbreviatedName := playerData.FirstName
		if len(playerData.FirstName) > 0 && len(playerData.LastName) > 0 {
			abbreviatedName = fmt.Sprintf("%c.%s", playerData.FirstName[0], playerData.LastName)
		}

		// Use name field from API response
		displayName := playerData.Name

		individual := &fetcher.Individual{
			VendorID:        playerData.ID,
			DisplayName:     displayName,
			AbbreviatedName: abbreviatedName,
			DateOfBirth:     dateOfBirth,
			Position:        playerData.Position,
			JerseyNumber:    playerData.JerseyNum,
			League:          league,
		}

		dataStore.AddIndividual(individual)
		roster.Players = append(roster.Players, individual)
	}

	dataStore.AddRoster(roster)
	return nil
}
