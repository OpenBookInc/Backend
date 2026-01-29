package fetcher

import (
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// Fetcher Data Store
// =============================================================================
// These structs are used by the fetcher layer to store data from the Sportradar
// API. They mirror the API response format and do not depend on shared/models.
//
// Key differences from shared/models:
// - No database ID fields (those come from DB after persistence)
// - Status fields are raw strings (enum transformation happens in persister)
// - Pointer relationships are maintained for in-memory navigation
// =============================================================================

// League represents a sports league (NFL, NBA, etc.)
type League struct {
	SportID int    // References sports table (1=NFL, 2=NBA)
	Name    string // e.g., "NFL", "NBA"

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the League
func (l *League) String() string {
	return fmt.Sprintf("\n%s (ID: %d, Sport ID: %d)\n", l.Name, l.ID, l.SportID)
}

// Conference represents a conference in a sports league
type Conference struct {
	VendorID string  // Sportradar UUID
	Name     string  // e.g., "AFC", "NFC"
	Alias    string  // Short name
	League   *League // Pointer to parent League

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Conference
func (c *Conference) String() string {
	leagueName := "Unknown"
	if c.League != nil {
		leagueName = c.League.Name
	}
	return fmt.Sprintf("\n%s (%s) - League: %s\n  ID: %d | Vendor ID: %s\n",
		c.Name, c.Alias, leagueName, c.ID, c.VendorID)
}

// Division represents a division within a conference
type Division struct {
	VendorID   string      // Sportradar UUID
	Name       string      // e.g., "AFC East", "NFC North"
	Alias      string      // Short name
	Conference *Conference // Pointer to parent Conference

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Division
func (d *Division) String() string {
	conferenceName := "Unknown"
	if d.Conference != nil {
		conferenceName = d.Conference.Name
	}
	return fmt.Sprintf("\n%s (%s) - Conference: %s\n  ID: %d | Vendor ID: %s\n",
		d.Name, d.Alias, conferenceName, d.ID, d.VendorID)
}

// Team represents a sports team
type Team struct {
	VendorID        string    // Sportradar UUID
	VendorUnifiedID string    // Sportradar unified ID (e.g., "sr:competitor:4418")
	Name            string    // Team name (e.g., "Cowboys")
	Market          string    // City/region (e.g., "Dallas")
	Alias           string    // Short name (e.g., "DAL")
	VenueName       string    // Venue name
	VenueCity       string    // Venue city
	VenueState      string    // Venue state
	Division        *Division // Pointer to parent Division

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Team
func (t *Team) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("  ID: %d | Vendor ID: %s | Unified ID: %s\n", t.ID, t.VendorID, t.VendorUnifiedID))
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
	VendorID        string     // Sportradar UUID
	VendorUnifiedID string     // Sportradar unified ID (e.g., "sr:player:2631629")
	DisplayName     string     // Full display name
	AbbreviatedName string     // Short name (e.g., "J.Smith")
	DateOfBirth     *time.Time // Can be null
	Position        string     // e.g., "QB", "PG"
	JerseyNumber    string     // Jersey number as string
	League          *League    // Pointer to parent League

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Individual
func (i *Individual) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s (#%s) - %s\n", i.DisplayName, i.JerseyNumber, i.Position))
	sb.WriteString(fmt.Sprintf("  ID: %d | Vendor ID: %s | Unified ID: %s\n", i.ID, i.VendorID, i.VendorUnifiedID))
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
	Team    *Team         // Pointer to parent Team
	Players []*Individual // Players on the roster

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Roster
func (r *Roster) String() string {
	var sb strings.Builder
	teamName := "Unknown"
	if r.Team != nil {
		teamName = fmt.Sprintf("%s %s", r.Team.Market, r.Team.Name)
	}
	sb.WriteString(fmt.Sprintf("\n%s Roster (ID: %d)\n", teamName, r.ID))
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
	VendorID           string    // Sportradar UUID
	ScheduledStartTime time.Time // Game start time
	HomeTeam           *Team     // Pointer to home team
	AwayTeam           *Team     // Pointer to away team

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the Game
func (g *Game) String() string {
	var sb strings.Builder
	homeTeamName := "Unknown"
	awayTeamName := "Unknown"
	if g.HomeTeam != nil {
		homeTeamName = fmt.Sprintf("%s %s", g.HomeTeam.Market, g.HomeTeam.Name)
	}
	if g.AwayTeam != nil {
		awayTeamName = fmt.Sprintf("%s %s", g.AwayTeam.Market, g.AwayTeam.Name)
	}
	sb.WriteString(fmt.Sprintf("\n%s vs %s (ID: %d)\n", homeTeamName, awayTeamName, g.ID))
	sb.WriteString(fmt.Sprintf("  Vendor ID: %s\n", g.VendorID))
	sb.WriteString(fmt.Sprintf("  Scheduled: %s\n", g.ScheduledStartTime.Format("2006-01-02 15:04:05 MST")))
	return sb.String()
}

// IndividualStatus represents the current status of a player
type IndividualStatus struct {
	Individual *Individual // Pointer to individual
	Status     string      // Raw API status string (e.g., "Questionable", "Out")

	// Set after persistence (for FK relationships)
	ID int
}

// String returns a formatted string representation of the IndividualStatus
func (is *IndividualStatus) String() string {
	individualName := "Unknown"
	if is.Individual != nil {
		individualName = is.Individual.DisplayName
	}
	return fmt.Sprintf("\n%s - Status: %s (ID: %d)\n", individualName, is.Status, is.ID)
}

// =============================================================================
// Reference Data
// =============================================================================

// ReferenceData holds all fetched data in memory
type ReferenceData struct {
	Leagues            map[int]*League              // Key: league sport ID (1=NFL, 2=NBA)
	Conferences        map[string]*Conference       // Key: vendor_id
	Divisions          map[string]*Division         // Key: vendor_id
	Teams              map[string]*Team             // Key: vendor_id
	Individuals        map[string]*Individual       // Key: vendor_id
	Rosters            map[string]*Roster           // Key: team vendor_id (one roster per team)
	Games              map[string]*Game             // Key: vendor_id (game ID)
	IndividualStatuses map[string]*IndividualStatus // Key: individual vendor_id (one status per player)
}

// NewReferenceData creates a new in-memory data store
func NewReferenceData() *ReferenceData {
	return &ReferenceData{
		Leagues:            make(map[int]*League),
		Conferences:        make(map[string]*Conference),
		Divisions:          make(map[string]*Division),
		Teams:              make(map[string]*Team),
		Individuals:        make(map[string]*Individual),
		Rosters:            make(map[string]*Roster),
		Games:              make(map[string]*Game),
		IndividualStatuses: make(map[string]*IndividualStatus),
	}
}

// AddLeague adds a league to the data store
func (ds *ReferenceData) AddLeague(league *League) {
	ds.Leagues[league.SportID] = league
}

// AddConference adds a conference to the data store
func (ds *ReferenceData) AddConference(conference *Conference) {
	ds.Conferences[conference.VendorID] = conference
}

// AddDivision adds a division to the data store
func (ds *ReferenceData) AddDivision(division *Division) {
	ds.Divisions[division.VendorID] = division
}

// AddTeam adds a team to the data store
func (ds *ReferenceData) AddTeam(team *Team) {
	ds.Teams[team.VendorID] = team
}

// AddIndividual adds an individual to the data store
func (ds *ReferenceData) AddIndividual(individual *Individual) {
	ds.Individuals[individual.VendorID] = individual
}

// AddRoster adds a roster to the data store
// Requires roster.Team to be set (will panic if nil)
func (ds *ReferenceData) AddRoster(roster *Roster) {
	ds.Rosters[roster.Team.VendorID] = roster
}

// AddGame adds a game to the data store
func (ds *ReferenceData) AddGame(game *Game) {
	ds.Games[game.VendorID] = game
}

// AddIndividualStatus adds or updates an individual's status in the data store
// Uses individual vendor_id as key for upsert behavior (only one status per player)
func (ds *ReferenceData) AddIndividualStatus(status *IndividualStatus) {
	if status.Individual != nil {
		ds.IndividualStatuses[status.Individual.VendorID] = status
	}
}

// GetLeagueByName returns a league by name (e.g., "NFL", "NBA")
func (ds *ReferenceData) GetLeagueByName(name string) *League {
	for _, league := range ds.Leagues {
		if league.Name == name {
			return league
		}
	}
	return nil
}
