package models

import "fmt"

// Division represents a division within a conference
type Division struct {
	ID           int         `json:"id"`             // Database ID (auto-increment)
	Name         string      `json:"name"`           // e.g., "AFC East", "NFC North"
	ConferenceID int64       `json:"conference_id"`  // Foreign key to conferences table
	VendorID     string      `json:"vendor_id"`      // Sportradar UUID
	Alias        string      `json:"alias"`          // Short name
	Conference   *Conference `json:"-"`              // Pointer to parent Conference (not stored in DB)
}

// String returns a formatted string representation of the Division
func (d *Division) String() string {
	conferenceName := "Unknown"
	if d.Conference != nil {
		conferenceName = d.Conference.Name
	}
	return fmt.Sprintf("\n%s (%s) - Conference: %s\n  DB ID: %d | Vendor ID: %s\n",
		d.Name, d.Alias, conferenceName, d.ID, d.VendorID)
}
