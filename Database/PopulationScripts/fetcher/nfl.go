package fetcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/models"
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

// NFLTeamRosterResponse represents the NFL team roster API response
type NFLTeamRosterResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Market  string `json:"market"`
	Alias   string `json:"alias"`
	Players []struct {
		ID          string      `json:"id"`
		FirstName   string      `json:"first_name"`
		LastName    string      `json:"last_name"`
		Position    string      `json:"position"`
		JerseyNum   string      `json:"jersey"`
		Height      interface{} `json:"height"` // Can be int or float
		Weight      interface{} `json:"weight"` // Can be int or float
		BirthDate   string      `json:"birth_date"`
		Status      string      `json:"status"`
	} `json:"players"`
}

// FetchNFLData fetches all NFL teams and rosters
func FetchNFLData(apiClient *client.Client, dataStore *models.DataStore) error {
	// Fetch teams
	teamsData, err := apiClient.GetNFLTeams()
	if err != nil {
		return fmt.Errorf("failed to fetch NFL teams: %w", err)
	}

	var hierarchyResp NFLHierarchyResponse
	if err := json.Unmarshal(teamsData, &hierarchyResp); err != nil {
		return fmt.Errorf("failed to parse NFL teams response: %w", err)
	}

	// Process conferences, divisions, and teams
	teamVendorIDs := []string{}
	for _, conferenceData := range hierarchyResp.Conferences {
		// Add conference (LeagueID will be set during persistence in main.go)
		conference := &models.Conference{
			Name:     conferenceData.Name,
			VendorID: conferenceData.ID,
			Alias:    conferenceData.Alias,
		}
		dataStore.AddConference(conference)

		for _, divisionData := range conferenceData.Divisions {
			// Add division (ConferenceID will be set during persistence in main.go)
			division := &models.Division{
				Name:     divisionData.Name,
				VendorID: divisionData.ID,
				Alias:    divisionData.Alias,
			}
			division.Conference = conference // Set pointer relationship
			dataStore.AddDivision(division)

			for _, teamData := range divisionData.Teams {
				// Add team (DivisionID will be set during persistence in main.go)
				team := &models.Team{
					Name:       teamData.Name,
					Market:     teamData.Market,
					Alias:      teamData.Alias,
					VendorID:   teamData.ID,
					VenueName:  teamData.Venue.Name,
					VenueCity:  teamData.Venue.City,
					VenueState: teamData.Venue.State,
				}
				team.Division = division // Set pointer relationship
				dataStore.AddTeam(team)
				teamVendorIDs = append(teamVendorIDs, teamData.ID)
			}
		}
	}

	// Wait before starting roster fetches to respect rate limits
	apiClient.RateLimitWait()

	// Fetch rosters for each team
	for _, teamVendorID := range teamVendorIDs {
		// Rate limiting - wait before each API request
		apiClient.RateLimitWait()

		if err := fetchNFLTeamRoster(apiClient, dataStore, teamVendorID); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamVendorID, err)
		}
	}

	return nil
}

func fetchNFLTeamRoster(apiClient *client.Client, dataStore *models.DataStore, teamVendorID string) error {
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

	roster := &models.Roster{
		Team:    team,
		Players: []*models.Individual{},
		// TeamID and IndividualIDs will be set during persistence in main.go
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

		individual := &models.Individual{
			VendorID:        playerData.ID,
			DisplayName:     fmt.Sprintf("%s %s", playerData.FirstName, playerData.LastName),
			AbbreviatedName: abbreviatedName,
			DateOfBirth:     dateOfBirth,
			Position:        playerData.Position,
			JerseyNumber:    playerData.JerseyNum,
			// LeagueID will be set during persistence in main.go
		}

		dataStore.AddIndividual(individual)
		roster.Players = append(roster.Players, individual)
	}

	dataStore.AddRoster(roster)
	return nil
}
