package nba

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbook/population-scripts/client/sportradar"
)

// PlayerProfile represents the NBA player profile API response.
// Only includes fields needed for individual persistence.
type PlayerProfile struct {
	ID           string `json:"id"`
	SrID         string `json:"sr_id"`
	FullName     string `json:"full_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	JerseyNumber string `json:"jersey_number"`
	Position     string `json:"position"`
	BirthDate    string `json:"birthdate"`
}

// GetDisplayName returns the player's display name
func (p *PlayerProfile) GetDisplayName() string {
	if p.FullName != "" {
		return p.FullName
	}
	return fmt.Sprintf("%s %s", p.FirstName, p.LastName)
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

// FetchPlayerProfile fetches profile data for a specific NBA player
func FetchPlayerProfile(apiClient *sportradar.Client, playerVendorID string) (*PlayerProfile, error) {
	data, err := apiClient.GetNBAPlayerProfile(playerVendorID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NBA player profile: %w", err)
	}

	var profile PlayerProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse NBA player profile response: %w", err)
	}

	return &profile, nil
}
