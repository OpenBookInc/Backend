package nfl

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
)

// PlayerProfile represents the NFL player profile API response.
// Only includes fields needed for individual persistence.
type PlayerProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Jersey    string `json:"jersey"`
	Position  string `json:"position"`
	BirthDate string `json:"birth_date"`
}

// GetDisplayName returns the player's display name
func (p *PlayerProfile) GetDisplayName() string {
	return p.Name
}

// GetAbbreviatedName returns the player's abbreviated name (e.g., "J.Smith")
func (p *PlayerProfile) GetAbbreviatedName() string {
	if len(p.FirstName) > 0 && len(p.LastName) > 0 {
		return fmt.Sprintf("%c.%s", p.FirstName[0], p.LastName)
	}
	return p.FirstName
}

// GetDateOfBirth parses and returns the player's date of birth
func (p *PlayerProfile) GetDateOfBirth() *time.Time {
	if p.BirthDate == "" {
		return nil
	}
	parsedDate, err := time.Parse("2006-01-02", p.BirthDate)
	if err != nil {
		return nil
	}
	return &parsedDate
}

// FetchPlayerProfile fetches profile data for a specific NFL player
func FetchPlayerProfile(apiClient *sportradar.Client, playerSportradarID string) (*PlayerProfile, error) {
	data, err := apiClient.GetNFLPlayerProfile(playerSportradarID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL player profile: %w", err)
	}

	var profile PlayerProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse NFL player profile response: %w", err)
	}

	return &profile, nil
}
