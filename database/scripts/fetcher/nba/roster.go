package nba

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/fetcher"
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
func FetchNBATeamRoster(apiClient *client.Client, dataStore *fetcher.ReferenceData, teamVendorID string, league *fetcher.League) error {
	rosterData, err := apiClient.GetNBATeamRoster(teamVendorID)
	if err != nil {
		return err
	}

	var profileResp NBATeamProfileResponse
	if err := json.Unmarshal(rosterData, &profileResp); err != nil {
		return fmt.Errorf("failed to parse team roster response: %w", err)
	}

	// Get the team from dataStore to link roster
	team := dataStore.Teams[teamVendorID]

	roster := &fetcher.Roster{
		Team:    team,
		Players: []*fetcher.Individual{},
	}

	for _, playerData := range profileResp.Players {
		// Parse birth date
		var dateOfBirth *time.Time
		if playerData.BirthDate != "" {
			if parsedDate, err := time.Parse("2006-01-02", playerData.BirthDate); err == nil {
				dateOfBirth = &parsedDate
			}
		}

		// Use FullName if available, otherwise construct from first/last
		displayName := playerData.FullName
		if displayName == "" {
			displayName = fmt.Sprintf("%s %s", playerData.FirstName, playerData.LastName)
		}

		// Create abbreviated name (e.g., "J.Smith" from "John Smith")
		abbreviatedName := playerData.FirstName
		if len(playerData.FirstName) > 0 && len(playerData.LastName) > 0 {
			abbreviatedName = fmt.Sprintf("%c.%s", playerData.FirstName[0], playerData.LastName)
		}

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
