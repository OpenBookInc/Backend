package models

import (
	"fmt"

	"github.com/openbook/shared/models/gen"
)

// IndividualStatus represents the current status of a player
type IndividualStatus struct {
	ID           int                `json:"id"`             // Database ID (auto-increment)
	IndividualID int64              `json:"individual_id"`  // Foreign key to individuals table
	Status       gen.IndividualStatus `json:"status"`         // Individual status enum (Active, Day To Day, Doubtful, Out, Out For Season, Questionable)
	Individual   *Individual        `json:"-"`              // Pointer to individual (not stored in DB)
}

// String returns a formatted string representation of the IndividualStatus
func (is *IndividualStatus) String() string {
	individualName := "Unknown"
	if is.Individual != nil {
		individualName = is.Individual.DisplayName
	}
	return fmt.Sprintf("\n%s - Status: %s (DB ID: %d, Individual ID: %d)\n",
		individualName, string(is.Status), is.ID, is.IndividualID)
}
