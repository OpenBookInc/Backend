package models

import "fmt"

// Conference represents a conference in a sports league
type Conference struct {
	ID       int      `json:"id"`        // Database ID (auto-increment)
	Name     string   `json:"name"`      // e.g., "AFC", "NFC"
	LeagueID int64    `json:"league_id"` // Foreign key to leagues table
	VendorID string   `json:"vendor_id"` // Sportradar UUID
	Alias    string   `json:"alias"`     // Short name
	League   *League  `json:"-"`         // Pointer to parent League (not stored in DB)
}

// String returns a formatted string representation of the Conference
func (c *Conference) String() string {
	leagueName := "Unknown"
	if c.League != nil {
		leagueName = c.League.Name
	}
	return fmt.Sprintf("\n%s (%s) - League: %s\n  DB ID: %d | Vendor ID: %s\n",
		c.Name, c.Alias, leagueName, c.ID, c.VendorID)
}
