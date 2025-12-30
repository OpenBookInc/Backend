package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/openbook/shared/models/gen"
)

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

// Roster represents a team's roster
type Roster struct {
	ID            int           `json:"id"`              // Database ID (auto-increment)
	TeamID        int64         `json:"team_id"`         // Foreign key to teams table
	IndividualIDs []int64       `json:"individual_ids"`  // Array of individual IDs (for DB persistence)
	Team          *Team         `json:"-"`               // Pointer to parent Team (not stored in DB)
	Players       []*Individual `json:"players"`         // Full player objects (in-memory only)
}

// String returns a formatted string representation of the Roster
func (r *Roster) String() string {
	var sb strings.Builder
	teamName := "Unknown"
	if r.Team != nil {
		teamName = fmt.Sprintf("%s %s", r.Team.Market, r.Team.Name)
	}
	sb.WriteString(fmt.Sprintf("\n%s Roster (DB ID: %d)\n", teamName, r.ID))
	sb.WriteString(fmt.Sprintf("  Team ID: %d\n", r.TeamID))
	sb.WriteString(fmt.Sprintf("  Player Count: %d\n", len(r.Players)))
	sb.WriteString("  Players:\n")
	for _, player := range r.Players {
		if player != nil {
			sb.WriteString(fmt.Sprintf("    - %s (#%s) - %s\n",
				player.DisplayName, player.JerseyNumber, player.Position))
		}
	}
	return sb.String()
}

// Game represents a scheduled or completed game
type Game struct {
	ID                   int       `json:"id"`                      // Database ID (auto-increment)
	ContenderIDA         int64     `json:"contender_id_a"`          // Foreign key to teams table (home team)
	ContenderIDB         int64     `json:"contender_id_b"`          // Foreign key to teams table (away team)
	VendorID             string    `json:"vendor_id"`               // Sportradar UUID
	ScheduledStartTime   time.Time `json:"scheduled_start_time"`    // Game start time
	TeamA                *Team     `json:"-"`                       // Pointer to home team (not stored in DB)
	TeamB                *Team     `json:"-"`                       // Pointer to away team (not stored in DB)
}

// String returns a formatted string representation of the Game
func (g *Game) String() string {
	var sb strings.Builder
	teamAName := "Unknown"
	teamBName := "Unknown"
	if g.TeamA != nil {
		teamAName = fmt.Sprintf("%s %s", g.TeamA.Market, g.TeamA.Name)
	}
	if g.TeamB != nil {
		teamBName = fmt.Sprintf("%s %s", g.TeamB.Market, g.TeamB.Name)
	}
	sb.WriteString(fmt.Sprintf("\n%s vs %s (DB ID: %d)\n", teamAName, teamBName, g.ID))
	sb.WriteString(fmt.Sprintf("  Vendor ID: %s\n", g.VendorID))
	sb.WriteString(fmt.Sprintf("  Scheduled: %s\n", g.ScheduledStartTime.Format("2006-01-02 15:04:05 MST")))
	return sb.String()
}

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
