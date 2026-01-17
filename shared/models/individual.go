package models

import (
	"fmt"
	"strings"
	"time"
)

// Individual represents a player/individual athlete
type Individual struct {
	ID               int       `json:"id"`                // Database ID (auto-increment)
	DisplayName      string    `json:"display_name"`      // Full display name
	AbbreviatedName  string    `json:"abbreviated_name"`  // Short name
	DateOfBirth      *time.Time `json:"date_of_birth"`    // Can be null in DB
	VendorID         string    `json:"vendor_id"`         // Sportradar UUID
	LeagueID         int64     `json:"league_id"`         // Foreign key to leagues table
	Position         string    `json:"position"`          // e.g., "QB", "PG"
	JerseyNumber     string    `json:"jersey_number"`     // Jersey number as string
	League           *League   `json:"-"`                 // Pointer to parent League (not stored in DB)
}

// String returns a formatted string representation of the Individual
func (i *Individual) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s (#%s) - %s\n", i.DisplayName, i.JerseyNumber, i.Position))
	sb.WriteString(fmt.Sprintf("  DB ID: %d | Vendor ID: %s\n", i.ID, i.VendorID))
	if i.DateOfBirth != nil {
		sb.WriteString(fmt.Sprintf("  Birth Date: %s\n", i.DateOfBirth.Format("2006-01-02")))
	}
	if i.League != nil {
		sb.WriteString(fmt.Sprintf("  League: %s\n", i.League.Name))
	}
	return sb.String()
}
