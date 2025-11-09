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
	teamIDs := []string{}
	for _, conferenceData := range hierarchyResp.Conferences {
		// Add conference
		conference := &models.Conference{
			ID:        conferenceData.ID,
			Name:      conferenceData.Name,
			Alias:     conferenceData.Alias,
			Sport:     models.SportNFL,
			CreatedAt: time.Now(),
		}
		dataStore.AddConference(conference)

		for _, divisionData := range conferenceData.Divisions {
			// Add division
			division := &models.Division{
				ID:           divisionData.ID,
				Name:         divisionData.Name,
				Alias:        divisionData.Alias,
				Sport:        models.SportNFL,
				ConferenceID: conferenceData.ID,
				CreatedAt:    time.Now(),
			}
			dataStore.AddDivision(division)

			for _, teamData := range divisionData.Teams {
				// Add team
				team := &models.Team{
					ID:           teamData.ID,
					Name:         teamData.Name,
					Market:       teamData.Market,
					Alias:        teamData.Alias,
					Sport:        models.SportNFL,
					ConferenceID: conferenceData.ID,
					Conference:   conferenceData.Name,
					DivisionID:   divisionData.ID,
					Division:     divisionData.Name,
					Venue: &models.Venue{
						ID:       teamData.Venue.ID,
						Name:     teamData.Venue.Name,
						City:     teamData.Venue.City,
						State:    teamData.Venue.State,
						Capacity: teamData.Venue.Capacity,
					},
					CreatedAt: time.Now(),
				}
				dataStore.AddTeam(team)
				teamIDs = append(teamIDs, team.ID)
			}
		}
	}

	// Wait before starting roster fetches to respect rate limits
	apiClient.RateLimitWait()

	// Fetch rosters for each team
	for _, teamID := range teamIDs {
		// Rate limiting - wait before each API request
		apiClient.RateLimitWait()

		if err := fetchNFLTeamRoster(apiClient, dataStore, teamID); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamID, err)
		}
	}

	return nil
}

func fetchNFLTeamRoster(apiClient *client.Client, dataStore *models.DataStore, teamID string) error {
	rosterData, err := apiClient.GetNFLTeamRoster(teamID)
	if err != nil {
		return err
	}

	var rosterResp NFLTeamRosterResponse
	if err := json.Unmarshal(rosterData, &rosterResp); err != nil {
		return fmt.Errorf("failed to parse team roster response: %w", err)
	}

	roster := &models.Roster{
		ID:        teamID,
		TeamID:    teamID,
		Sport:     models.SportNFL,
		Season:    "current",
		Players:   []models.Individual{},
		UpdatedAt: time.Now(),
	}

	for _, playerData := range rosterResp.Players {
		// Parse height and weight (can be int or float from API)
		height := parseHeight(playerData.Height)
		weight := parseWeight(playerData.Weight)

		individual := models.Individual{
			ID:           playerData.ID,
			FirstName:    playerData.FirstName,
			LastName:     playerData.LastName,
			FullName:     fmt.Sprintf("%s %s", playerData.FirstName, playerData.LastName),
			Sport:        models.SportNFL,
			Position:     playerData.Position,
			JerseyNumber: playerData.JerseyNum,
			Height:       height,
			Weight:       weight,
			BirthDate:    playerData.BirthDate,
			Status:       playerData.Status,
			CreatedAt:    time.Now(),
		}

		dataStore.AddIndividual(&individual)
		roster.Players = append(roster.Players, individual)
	}

	dataStore.AddRoster(roster)
	return nil
}
