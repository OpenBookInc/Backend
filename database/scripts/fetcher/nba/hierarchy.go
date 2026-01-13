package nba

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client/sportradar"
	"github.com/openbook/population-scripts/fetcher"
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

// FetchNBAHierarchyData fetches all NBA teams and rosters
func FetchNBAHierarchyData(apiClient *sportradar.Client, dataStore *fetcher.ReferenceData) error {
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
		conference := &fetcher.Conference{
			Name:     conferenceData.Name,
			VendorID: conferenceData.ID,
			Alias:    conferenceData.Alias,
			League:   nbaLeague,
		}
		dataStore.AddConference(conference)

		for _, divisionData := range conferenceData.Divisions {
			division := &fetcher.Division{
				Name:       divisionData.Name,
				VendorID:   divisionData.ID,
				Alias:      divisionData.Alias,
				Conference: conference,
			}
			dataStore.AddDivision(division)

			for _, teamData := range divisionData.Teams {
				team := &fetcher.Team{
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

		if err := FetchNBATeamRoster(apiClient, dataStore, teamVendorID, nbaLeague); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamVendorID, err)
		}
	}

	return nil
}
