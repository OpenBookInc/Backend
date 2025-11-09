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
	teamIDs := []string{}
	for _, conferenceData := range hierarchyResp.Conferences {
		// Add conference
		conference := &models.Conference{
			ID:        conferenceData.ID,
			Name:      conferenceData.Name,
			Alias:     conferenceData.Alias,
			Sport:     models.SportNBA,
			CreatedAt: time.Now(),
		}
		dataStore.AddConference(conference)

		for _, divisionData := range conferenceData.Divisions {
			// Add division
			division := &models.Division{
				ID:           divisionData.ID,
				Name:         divisionData.Name,
				Alias:        divisionData.Alias,
				Sport:        models.SportNBA,
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
					Sport:        models.SportNBA,
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

		if err := fetchNBATeamRoster(apiClient, dataStore, teamID); err != nil {
			return fmt.Errorf("failed to fetch roster for team %s: %w", teamID, err)
		}
	}

	return nil
}

func fetchNBATeamRoster(apiClient *client.Client, dataStore *models.DataStore, teamID string) error {
	rosterData, err := apiClient.GetNBATeamRoster(teamID)
	if err != nil {
		return err
	}

	var profileResp NBATeamProfileResponse
	if err := json.Unmarshal(rosterData, &profileResp); err != nil {
		return fmt.Errorf("failed to parse team roster response: %w", err)
	}

	roster := &models.Roster{
		ID:        teamID,
		TeamID:    teamID,
		Sport:     models.SportNBA,
		Season:    "current",
		Players:   []models.Individual{},
		UpdatedAt: time.Now(),
	}

	for _, playerData := range profileResp.Players {
		// Parse height and weight
		height := parseHeight(playerData.Height)
		weight := parseWeight(playerData.Weight)

		individual := models.Individual{
			ID:           playerData.ID,
			FirstName:    playerData.FirstName,
			LastName:     playerData.LastName,
			FullName:     playerData.FullName,
			Sport:        models.SportNBA,
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
