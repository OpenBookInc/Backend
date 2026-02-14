package models

import (
	"fmt"
	"strings"
)

// Team represents a sports team
type Team struct {
	ID         int       `json:"id"`          // Database ID (auto-increment)
	Name       string    `json:"name"`        // Team name (e.g., "Cowboys")
	Market     string    `json:"market"`      // City/region (e.g., "Dallas")
	Alias      string    `json:"alias"`       // Short name (e.g., "DAL")
	VendorID   string    `json:"vendor_id"`   // Sportradar UUID
	DivisionID int64     `json:"division_id"` // Foreign key to divisions table
	VenueName  string    `json:"venue_name"`  // Venue name
	VenueCity  string    `json:"venue_city"`  // Venue city
	VenueState string    `json:"venue_state"` // Venue state
	Division   *Division `json:"-"`           // Pointer to parent Division (not stored in DB)
}

// String returns a formatted string representation of the Team
func (t *Team) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("  DB ID: %d | Vendor ID: %s\n", t.ID, t.VendorID))
	if t.Division != nil {
		sb.WriteString(fmt.Sprintf("  Division: %s", t.Division.Name))
		if t.Division.Conference != nil {
			sb.WriteString(fmt.Sprintf(" (%s)", t.Division.Conference.Name))
		}
		sb.WriteString("\n")
	}
	if t.VenueName != "" {
		sb.WriteString(fmt.Sprintf("  Venue: %s, %s, %s\n", t.VenueName, t.VenueCity, t.VenueState))
	}
	return sb.String()
}
