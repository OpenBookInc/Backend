package models

import (
	"fmt"
	"strings"
	"time"
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

// DataStore holds all sports data in memory
type DataStore struct {
	Leagues           map[int]*League               // Key: league DB ID
	Conferences       map[string]*Conference        // Key: vendor_id
	Divisions         map[string]*Division          // Key: vendor_id
	Teams             map[string]*Team              // Key: vendor_id
	Individuals       map[string]*Individual        // Key: vendor_id
	Rosters           map[string]*Roster            // Key: team vendor_id (one roster per team)
	Games             map[string]*Game              // Key: vendor_id (game ID)
	IndividualStatuses map[string]*IndividualStatus // Key: individual vendor_id (one status per player)
}

// NewDataStore creates a new in-memory data store
func NewDataStore() *DataStore {
	return &DataStore{
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
func (ds *DataStore) AddLeague(league *League) {
	ds.Leagues[league.ID] = league
}

// AddConference adds a conference to the data store
func (ds *DataStore) AddConference(conference *Conference) {
	ds.Conferences[conference.VendorID] = conference
}

// AddDivision adds a division to the data store
func (ds *DataStore) AddDivision(division *Division) {
	ds.Divisions[division.VendorID] = division
}

// AddTeam adds a team to the data store
func (ds *DataStore) AddTeam(team *Team) {
	ds.Teams[team.VendorID] = team
}

// AddIndividual adds an individual to the data store
func (ds *DataStore) AddIndividual(individual *Individual) {
	ds.Individuals[individual.VendorID] = individual
}

// AddRoster adds a roster to the data store
// Requires roster.Team to be set (will panic if nil)
func (ds *DataStore) AddRoster(roster *Roster) {
	ds.Rosters[roster.Team.VendorID] = roster
}

// AddGame adds a game to the data store
func (ds *DataStore) AddGame(game *Game) {
	ds.Games[game.VendorID] = game
}

// AddIndividualStatus adds or updates an individual's status in the data store
// Uses individual vendor_id as key for upsert behavior (only one status per player)
func (ds *DataStore) AddIndividualStatus(status *IndividualStatus) {
	if status.Individual != nil {
		ds.IndividualStatuses[status.Individual.VendorID] = status
	}
}

// GetLeagueByName returns a league by name (e.g., "NFL", "NBA")
func (ds *DataStore) GetLeagueByName(name string) *League {
	for _, league := range ds.Leagues {
		if league.Name == name {
			return league
		}
	}
	return nil
}

// GetConferencesByLeagueID returns all conferences for a given league
func (ds *DataStore) GetConferencesByLeagueID(leagueID int64) []*Conference {
	conferences := []*Conference{}
	for _, conference := range ds.Conferences {
		if conference.LeagueID == leagueID {
			conferences = append(conferences, conference)
		}
	}
	return conferences
}

// GetDivisionsByConferenceID returns all divisions for a given conference
func (ds *DataStore) GetDivisionsByConferenceID(conferenceID int64) []*Division {
	divisions := []*Division{}
	for _, division := range ds.Divisions {
		if division.ConferenceID == conferenceID {
			divisions = append(divisions, division)
		}
	}
	return divisions
}

// GetTeamsByDivisionID returns all teams for a given division
func (ds *DataStore) GetTeamsByDivisionID(divisionID int64) []*Team {
	teams := []*Team{}
	for _, team := range ds.Teams {
		if team.DivisionID == divisionID {
			teams = append(teams, team)
		}
	}
	return teams
}

// GetIndividualsByLeagueID returns all individuals for a given league
func (ds *DataStore) GetIndividualsByLeagueID(leagueID int64) []*Individual {
	individuals := []*Individual{}
	for _, individual := range ds.Individuals {
		if individual.LeagueID == leagueID {
			individuals = append(individuals, individual)
		}
	}
	return individuals
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
	ID           int                  `json:"id"`             // Database ID (auto-increment)
	IndividualID int64                `json:"individual_id"`  // Foreign key to individuals table
	Status       IndividualStatusType `json:"status"`         // Individual status enum (Active, Day To Day, Doubtful, Out, Out For Season, Questionable)
	Individual   *Individual          `json:"-"`              // Pointer to individual (not stored in DB)
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
