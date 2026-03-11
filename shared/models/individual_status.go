package models

import (
	"fmt"

	"github.com/openbook/shared/models/gen"
	"github.com/openbook/shared/utils"
)

// IndividualStatus represents the current status of a player
type IndividualStatus struct {
	IndividualID utils.UUID           `json:"individual_id"`  // Foreign key to individuals table (also PK)
	Status       gen.IndividualStatus `json:"status"`         // Individual status enum (Active, Day To Day, Doubtful, Out, Out For Season, Questionable)
	Individual   *Individual          `json:"-"`              // Pointer to individual (not stored in DB)
}

// String returns a formatted string representation of the IndividualStatus
func (is *IndividualStatus) String() string {
	individualName := "Unknown"
	if is.Individual != nil {
		individualName = is.Individual.DisplayName
	}
	return fmt.Sprintf("\n%s - Status: %s (Individual ID: %s)\n",
		individualName, string(is.Status), is.IndividualID.String())
}
