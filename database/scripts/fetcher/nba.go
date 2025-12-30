package fetcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client"
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
func FetchNBAData(apiClient *client.Client, dataStore *ReferenceData) error {
	// Fetch teams
	teamsData, err := apiClient.GetNBATeams()
	if err != nil {
		return fmt.Errorf("failed to fetch NBA teams: %w", err)
	}

	var hierarchyResp NBAHierarchyResponse
	if err := json.Unmarshal(teamsData, &hierarchyResp); err != nil {
		return fmt.Errorf("failed to parse NBA teams response: %w", err)
	}

	// Get NBA league from dataStore (must be added by caller first)
	nbaLeague := dataStore.GetLeagueByName("NBA")

	// Process conferences, divisions, and teams
	teamVendorIDs := []string{}
	for _, conferenceData := range hierarchyResp.Conferences {
		conference := &Conference{
			Name:     conferenceData.Name,
			VendorID: conferenceData.ID,
			Alias:    conferenceData.Alias,
			League:   nbaLeague,
		}
		dataStore.AddConference(conference)

		for _, divisionData := range conferenceData.Divisions {
			division := &Division{
				Name:       divisionData.Name,
				VendorID:   divisionData.ID,
				Alias:      divisionData.Alias,
				Conference: conference,
			}
			dataStore.AddDivision(division)

			for _, teamData := range divisionData.Teams {
				team := &Team{
					Name:       teamData.Name,
					Market:     teamData.Market,
					Alias:      teamData.Alias,
					VendorID:   teamData.ID,
					VenueName:  teamData.Venue.Name,
					VenueCity:  teamData.Venue.City,
					VenueState: teamData.Venue.State,
					Division:   division,
				}
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

		if err := fetchNBATeamRoster(apiClient, dataStore, teamVendorID, nbaLeague); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamVendorID, err)
		}
	}

	return nil
}

func fetchNBATeamRoster(apiClient *client.Client, dataStore *ReferenceData, teamVendorID string, league *League) error {
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

	roster := &Roster{
		Team:    team,
		Players: []*Individual{},
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

		individual := &Individual{
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
