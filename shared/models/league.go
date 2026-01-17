package models

import "fmt"

// League represents a sports league (NFL, NBA, etc.)
type League struct {
	ID      int    `json:"id"`       // Database ID (auto-increment)
	SportID int64  `json:"sport_id"` // References sports table
	Name    string `json:"name"`     // e.g., "NFL", "NBA"
}

// String returns a formatted string representation of the League
func (l *League) String() string {
	return fmt.Sprintf("\n%s (ID: %d, Sport ID: %d)\n", l.Name, l.ID, l.SportID)
}
