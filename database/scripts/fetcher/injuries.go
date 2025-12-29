package fetcher

import (
	"encoding/json"
	"fmt"

	"github.com/openbook/population-scripts/client"
	"github.com/openbook/population-scripts/persister"
	"github.com/openbook/shared/models"
	"github.com/openbook/shared/models/gen"
)

// NFLInjuriesResponse represents the NFL weekly injuries API response
type NFLInjuriesResponse struct {
	Teams []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Market  string `json:"market"`
		Alias   string `json:"alias"`
		Players []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Position string `json:"position"`
			Jersey   string `json:"jersey"`
			Injuries []struct {
				Status  string `json:"status"`  // e.g., "Out", "Questionable", "Doubtful" - game status
				Primary string `json:"primary"` // Injury description (e.g., "Hip", "Shoulder")
			} `json:"injuries"`
		} `json:"players"`
	} `json:"teams"`
}

// NBAInjuriesResponse represents the NBA injuries API response
type NBAInjuriesResponse struct {
	Teams []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Market  string `json:"market"`
		Players []struct {
			ID        string `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			FullName  string `json:"full_name"`
			Position  string `json:"position"`
			Injuries  []struct {
				Status  string `json:"status"`  // e.g., "Out", "Day To Day"
				Desc    string `json:"desc"`    // Injury description (e.g., "Hamstring", "Calf")
				Comment string `json:"comment"` // Additional details
			} `json:"injuries"`
		} `json:"players"`
	} `json:"teams"`
}

// FetchNFLPlayerStatuses fetches player injury statuses from NFL weekly injuries endpoint
func FetchNFLPlayerStatuses(apiClient *client.Client, dataStore *models.DataStore, year int, seasonType string, week int) error {
	injuriesData, err := apiClient.GetNFLWeeklyInjuries(year, seasonType, week)
	if err != nil {
		return fmt.Errorf("failed to fetch NFL weekly injuries: %w", err)
	}

	var injuriesResp NFLInjuriesResponse
	if err := json.Unmarshal(injuriesData, &injuriesResp); err != nil {
		return fmt.Errorf("failed to parse NFL injuries response: %w", err)
	}

	// Process all teams and their injured players
	for _, team := range injuriesResp.Teams {
		for _, playerData := range team.Players {
			// Get player from dataStore
			individual := dataStore.Individuals[playerData.ID]
			if individual == nil {
				// Player might not be in our roster data, skip
				continue
			}

			// Use injuries[0].status if available (game status like "Questionable", "Out", etc.)
			// If no status field exists, player is just on injury report but not game-status impacted
			statusStr := "Active"
			if len(playerData.Injuries) > 0 && playerData.Injuries[0].Status != "" {
				statusStr = playerData.Injuries[0].Status
			}

			// Map the status to database enum format
			mappedStatus, err := persister.MapIndividualStatusToDB(statusStr)
			if err != nil {
				return fmt.Errorf("invalid status for NFL player %s (%s): %w", playerData.Name, playerData.ID, err)
			}

			individualStatus := &models.IndividualStatus{
				Individual: individual,
				Status:     gen.IndividualStatus(mappedStatus),
				// IndividualID will be set during persistence in main.go
			}

			dataStore.AddIndividualStatus(individualStatus)
		}
	}

	return nil
}

// FetchNBAPlayerStatuses fetches player injury statuses from NBA injuries endpoint
func FetchNBAPlayerStatuses(apiClient *client.Client, dataStore *models.DataStore) error {
	injuriesData, err := apiClient.GetNBAInjuries()
	if err != nil {
		return fmt.Errorf("failed to fetch NBA injuries: %w", err)
	}

	var injuriesResp NBAInjuriesResponse
	if err := json.Unmarshal(injuriesData, &injuriesResp); err != nil {
		return fmt.Errorf("failed to parse NBA injuries response: %w", err)
	}

	// Process all teams and their injured players
	for _, team := range injuriesResp.Teams {
		for _, playerData := range team.Players {
			// Get player from dataStore
			individual := dataStore.Individuals[playerData.ID]
			if individual == nil {
				// Player might not be in our roster data, skip
				continue
			}

			// Use injuries[0].status if available (game status like "Day To Day", "Out", etc.)
			statusStr := "Active"
			if len(playerData.Injuries) > 0 && playerData.Injuries[0].Status != "" {
				statusStr = playerData.Injuries[0].Status
			}

			// Map the status to database enum format
			mappedStatus, err := persister.MapIndividualStatusToDB(statusStr)
			if err != nil {
				return fmt.Errorf("invalid status for NBA player %s (%s): %w", playerData.FullName, playerData.ID, err)
			}

			individualStatus := &models.IndividualStatus{
				Individual: individual,
				Status:     gen.IndividualStatus(mappedStatus),
				// IndividualID will be set during persistence in main.go
			}

			dataStore.AddIndividualStatus(individualStatus)
		}
	}

	return nil
}

// SetDefaultActiveStatuses sets all players without a status to "Active"
// This should be called after fetching injury data to ensure all players have a status
func SetDefaultActiveStatuses(dataStore *models.DataStore) {
	for _, individual := range dataStore.Individuals {
		// Check if this individual already has a status
		if _, exists := dataStore.IndividualStatuses[individual.VendorID]; !exists {
			// No status yet, set to Active
			individualStatus := &models.IndividualStatus{
				Individual: individual,
				Status:     gen.IndividualStatusActive,
				// IndividualID will be set during persistence in main.go
			}
			dataStore.AddIndividualStatus(individualStatus)
		}
	}
}
