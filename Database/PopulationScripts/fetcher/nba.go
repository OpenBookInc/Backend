package fetcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/models"
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

// FetchNBAData fetches all NBA teams and rosters
func FetchNBAData(apiClient *client.Client, dataStore *models.DataStore) error {
	// Fetch teams
	teamsData, err := apiClient.GetNBATeams()
	if err != nil {
		return fmt.Errorf("failed to fetch NBA teams: %w", err)
	}

	var hierarchyResp NBAHierarchyResponse
	if err := json.Unmarshal(teamsData, &hierarchyResp); err != nil {
		return fmt.Errorf("failed to parse NBA teams response: %w", err)
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

		if err := fetchNBATeamRoster(apiClient, dataStore, teamVendorID); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamVendorID, err)
		}
	}

	return nil
}

func fetchNBATeamRoster(apiClient *client.Client, dataStore *models.DataStore, teamVendorID string) error {
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

	roster := &models.Roster{
		Team:    team,
		Players: []*models.Individual{},
		// TeamID and IndividualIDs will be set during persistence in main.go
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

		individual := &models.Individual{
			VendorID:        playerData.ID,
			DisplayName:     displayName,
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
