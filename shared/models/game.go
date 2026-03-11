package models

import (
	"fmt"
	"time"

	"github.com/openbook/shared/utils"
)

// Game represents a scheduled or completed game
type Game struct {
	ID                   utils.UUID `json:"id"`                      // Database UUID
	ContenderIDA         utils.UUID `json:"contender_id_a"`          // Foreign key to teams table (home team)
	ContenderIDB         utils.UUID `json:"contender_id_b"`          // Foreign key to teams table (away team)
	SportradarID         string     `json:"sportradar_id"`           // Sportradar UUID
	ScheduledStartTime   time.Time  `json:"scheduled_start_time"`    // Game start time
	TeamA                *Team      `json:"-"`                       // Pointer to home team (not stored in DB)
	TeamB                *Team      `json:"-"`                       // Pointer to away team (not stored in DB)
}

// String returns a formatted string representation of the Game
func (g *Game) String() string {
	var sb string
	teamAName := "Unknown"
	teamBName := "Unknown"
	if g.TeamA != nil {
		teamAName = fmt.Sprintf("%s %s", g.TeamA.Market, g.TeamA.Name)
	}
	if g.TeamB != nil {
		teamBName = fmt.Sprintf("%s %s", g.TeamB.Market, g.TeamB.Name)
	}
	sb += fmt.Sprintf("\n%s vs %s (DB ID: %s)\n", teamAName, teamBName, g.ID.String())
	sb += fmt.Sprintf("  Sportradar ID: %s\n", g.SportradarID)
	sb += fmt.Sprintf("  Scheduled: %s\n", g.ScheduledStartTime.Format("2006-01-02 15:04:05 MST"))
	return sb
}
