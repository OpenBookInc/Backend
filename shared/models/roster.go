package models

import (
	"fmt"

	"github.com/openbook/shared/utils"
)

// Roster represents a team's roster
type Roster struct {
	TeamID        utils.UUID   `json:"team_id"`         // Foreign key to teams table (also PK)
	IndividualIDs []utils.UUID `json:"individual_ids"`  // Array of individual UUIDs (for DB persistence)
	Team          *Team        `json:"-"`               // Pointer to parent Team (not stored in DB)
	Players       []*Individual `json:"players"`        // Full player objects (in-memory only)
}

// String returns a formatted string representation of the Roster
func (r *Roster) String() string {
	var sb string
	teamName := "Unknown"
	if r.Team != nil {
		teamName = fmt.Sprintf("%s %s", r.Team.Market, r.Team.Name)
	}
	sb += fmt.Sprintf("\n%s Roster (Team ID: %s)\n", teamName, r.TeamID.String())
	sb += fmt.Sprintf("  Team ID: %s\n", r.TeamID.String())
	sb += fmt.Sprintf("  Player Count: %d\n", len(r.Players))
	sb += "  Players:\n"
	for _, player := range r.Players {
		if player != nil {
			sb += fmt.Sprintf("    - %s (#%s) - %s\n",
				player.DisplayName, player.JerseyNumber, player.Position)
		}
	}
	return sb
}
