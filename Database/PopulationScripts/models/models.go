package models

import (
	"fmt"
	"strings"
	"time"
)

// Sport represents the type of sport
type Sport string

const (
	SportNBA Sport = "NBA"
	SportNFL Sport = "NFL"
)

// Conference represents a conference in a sports league
type Conference struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Alias     string    `json:"alias"`
	Sport     Sport     `json:"sport"`
	CreatedAt time.Time `json:"created_at"`
}

// String returns a formatted string representation of the Conference
func (c *Conference) String() string {
	return fmt.Sprintf("\n%s (%s)\n  ID: %s\n  Sport: %s\n", c.Name, c.Alias, c.ID, c.Sport)
}

// Division represents a division within a conference
type Division struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Alias        string    `json:"alias"`
	Sport        Sport     `json:"sport"`
	ConferenceID string    `json:"conference_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// String returns a formatted string representation of the Division
func (d *Division) String() string {
	return fmt.Sprintf("\n%s (%s)\n  ID: %s\n  Conference ID: %s\n  Sport: %s\n", d.Name, d.Alias, d.ID, d.ConferenceID, d.Sport)
}

// Team represents a sports team across all sports
type Team struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Market       string    `json:"market"`       // City or region (e.g., "Los Angeles")
	Alias        string    `json:"alias"`        // Short name/abbreviation (e.g., "LAL")
	Sport        Sport     `json:"sport"`
	ConferenceID string    `json:"conference_id"` // Conference ID reference
	Conference   string    `json:"conference"`    // Conference name (denormalized for convenience)
	DivisionID   string    `json:"division_id"`   // Division ID reference
	Division     string    `json:"division"`      // Division name (denormalized for convenience)
	Venue        *Venue    `json:"venue,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// String returns a formatted string representation of the Team
func (t *Team) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s %s (%s)\n", t.Market, t.Name, t.Alias))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("  Conference: %s\n", t.Conference))
	sb.WriteString(fmt.Sprintf("  Division: %s\n", t.Division))
	if t.Venue != nil {
		sb.WriteString(fmt.Sprintf("  Venue: %s\n", t.Venue.String()))
	}
	return sb.String()
}

// Individual represents a player across all sports
type Individual struct {
	ID           string    `json:"id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	FullName     string    `json:"full_name"`
	Sport        Sport     `json:"sport"`
	Position     string    `json:"position"`     // Generic position field
	JerseyNumber string    `json:"jersey_number"`
	Height       int       `json:"height"`       // Height in inches
	Weight       int       `json:"weight"`       // Weight in pounds
	BirthDate    string    `json:"birth_date"`   // ISO date string
	Status       string    `json:"status"`       // Active, Injured, etc.
	CreatedAt    time.Time `json:"created_at"`
}

// String returns a formatted string representation of the Individual
func (i *Individual) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s (#%s)\n", i.FullName, i.JerseyNumber))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", i.ID))
	sb.WriteString(fmt.Sprintf("  Position: %s\n", i.Position))
	sb.WriteString(fmt.Sprintf("  Height: %d inches | Weight: %d lbs\n", i.Height, i.Weight))
	sb.WriteString(fmt.Sprintf("  Birth Date: %s | Status: %s\n", i.BirthDate, i.Status))
	return sb.String()
}

// Roster represents a team's roster
type Roster struct {
	ID          string       `json:"id"`
	TeamID      string       `json:"team_id"`
	Sport       Sport        `json:"sport"`
	Season      string       `json:"season"`      // Season identifier (e.g., "2024")
	Players     []Individual `json:"players"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// String returns a formatted string representation of the Roster
func (r *Roster) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Roster ID: %s\n", r.ID))
	sb.WriteString(fmt.Sprintf("  Team ID: %s\n", r.TeamID))
	sb.WriteString(fmt.Sprintf("  Season: %s | Sport: %s\n", r.Season, r.Sport))
	sb.WriteString(fmt.Sprintf("  Player Count: %d\n", len(r.Players)))
	sb.WriteString(fmt.Sprintf("  Last Updated: %s\n", r.UpdatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("  Players:\n")
	for _, player := range r.Players {
		sb.WriteString(fmt.Sprintf("    - %s (#%s) - %s\n", player.FullName, player.JerseyNumber, player.Position))
	}
	return sb.String()
}

// StringWithTeamName returns a formatted string with team name included
func (r *Roster) StringWithTeamName(teamName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s - %s %s Roster\n", teamName, r.Season, r.Sport))
	sb.WriteString(r.String())
	return sb.String()
}

// Venue represents a sports venue
type Venue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	City     string `json:"city"`
	State    string `json:"state"`
	Capacity int    `json:"capacity"`
}

// String returns a formatted string representation of the Venue
func (v *Venue) String() string {
	return fmt.Sprintf("%s (%s, %s) - Capacity: %d", v.Name, v.City, v.State, v.Capacity)
}

// DataStore holds all sports data in memory
type DataStore struct {
	Conferences map[string]*Conference // Key: conference ID
	Divisions   map[string]*Division   // Key: division ID
	Teams       map[string]*Team       // Key: team ID
	Individuals map[string]*Individual // Key: individual ID
	Rosters     map[string]*Roster     // Key: roster ID
}

// NewDataStore creates a new in-memory data store
func NewDataStore() *DataStore {
	return &DataStore{
		Conferences: make(map[string]*Conference),
		Divisions:   make(map[string]*Division),
		Teams:       make(map[string]*Team),
		Individuals: make(map[string]*Individual),
		Rosters:     make(map[string]*Roster),
	}
}

// AddConference adds a conference to the data store
func (ds *DataStore) AddConference(conference *Conference) {
	ds.Conferences[conference.ID] = conference
}

// AddDivision adds a division to the data store
func (ds *DataStore) AddDivision(division *Division) {
	ds.Divisions[division.ID] = division
}

// AddTeam adds a team to the data store
func (ds *DataStore) AddTeam(team *Team) {
	ds.Teams[team.ID] = team
}

// AddIndividual adds an individual to the data store
func (ds *DataStore) AddIndividual(individual *Individual) {
	ds.Individuals[individual.ID] = individual
}

// AddRoster adds a roster to the data store
func (ds *DataStore) AddRoster(roster *Roster) {
	ds.Rosters[roster.ID] = roster
}

// GetConferencesBySport returns all conferences for a given sport
func (ds *DataStore) GetConferencesBySport(sport Sport) []*Conference {
	conferences := []*Conference{}
	for _, conference := range ds.Conferences {
		if conference.Sport == sport {
			conferences = append(conferences, conference)
		}
	}
	return conferences
}

// GetDivisionsBySport returns all divisions for a given sport
func (ds *DataStore) GetDivisionsBySport(sport Sport) []*Division {
	divisions := []*Division{}
	for _, division := range ds.Divisions {
		if division.Sport == sport {
			divisions = append(divisions, division)
		}
	}
	return divisions
}

// GetTeamsBySport returns all teams for a given sport
func (ds *DataStore) GetTeamsBySport(sport Sport) []*Team {
	teams := []*Team{}
	for _, team := range ds.Teams {
		if team.Sport == sport {
			teams = append(teams, team)
		}
	}
	return teams
}

// GetIndividualsBySport returns all individuals for a given sport
func (ds *DataStore) GetIndividualsBySport(sport Sport) []*Individual {
	individuals := []*Individual{}
	for _, individual := range ds.Individuals {
		if individual.Sport == sport {
			individuals = append(individuals, individual)
		}
	}
	return individuals
}
