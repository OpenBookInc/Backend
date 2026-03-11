package models

import (
	"fmt"
	"strings"
	"sync"

	"github.com/openbook/shared/models/gen"
	"github.com/openbook/shared/utils"
)

// vendorLookupKey is used for reverse lookups: (entity_type, vendor, vendor_id) → entity_id
type vendorLookupKey struct {
	EntityType gen.Entity
	Vendor     gen.Vendor
	VendorID   string
}

// entityVendorKey is used for forward lookups: (entity_type, entity_id, vendor) → vendor_id
type entityVendorKey struct {
	EntityType gen.Entity
	EntityID   utils.UUID
	Vendor     gen.Vendor
}

// ModelRegistry provides thread-safe storage and retrieval of model instances.
// It ensures that each entity (by database ID) has exactly one instance in memory,
// enabling pointer-based relationships between models.
//
// The registry uses slices to store data (for pointer stability when growing)
// and maps for O(1) lookups by database ID.
//
// Usage:
//   - Store methods should check the registry before querying the database
//   - After fetching from DB, register the entity and return the registered pointer
//   - Nested struct pointers should be resolved before registering parent entities
type ModelRegistry struct {
	mu sync.RWMutex

	// Slices own the data (appending doesn't invalidate existing pointers)
	leagues            []League
	conferences        []Conference
	divisions          []Division
	teams              []Team
	individuals        []Individual
	games              []Game
	rosters            []Roster
	individualStatuses []IndividualStatus

	// Maps for O(1) lookups by database ID (pointers into slices)
	leaguesByID            map[int]*League
	conferencesByID        map[int]*Conference
	divisionsByID          map[int]*Division
	teamsByID              map[utils.UUID]*Team
	individualsByID        map[utils.UUID]*Individual
	gamesByID              map[utils.UUID]*Game
	rostersByTeamID        map[utils.UUID]*Roster
	individualStatusesByIndividualID map[utils.UUID]*IndividualStatus

	// Vendor ID maps for cross-referencing entities across vendors
	vendorToEntityID map[vendorLookupKey]utils.UUID // (entity_type, vendor, vendor_id) → entity_id
	entityToVendorID map[entityVendorKey]string     // (entity_type, entity_id, vendor) → vendor_id
}

// Registry is the global singleton for model instances.
var Registry = NewModelRegistry()

// NewModelRegistry creates a new initialized ModelRegistry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		leaguesByID:            make(map[int]*League),
		conferencesByID:        make(map[int]*Conference),
		divisionsByID:          make(map[int]*Division),
		teamsByID:              make(map[utils.UUID]*Team),
		individualsByID:        make(map[utils.UUID]*Individual),
		gamesByID:              make(map[utils.UUID]*Game),
		rostersByTeamID:        make(map[utils.UUID]*Roster),
		individualStatusesByIndividualID: make(map[utils.UUID]*IndividualStatus),
		vendorToEntityID:       make(map[vendorLookupKey]utils.UUID),
		entityToVendorID:       make(map[entityVendorKey]string),
	}
}

// Clear resets all registry data. Useful for testing or resetting state.
func (r *ModelRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.leagues = nil
	r.conferences = nil
	r.divisions = nil
	r.teams = nil
	r.individuals = nil
	r.games = nil
	r.rosters = nil
	r.individualStatuses = nil

	r.leaguesByID = make(map[int]*League)
	r.conferencesByID = make(map[int]*Conference)
	r.divisionsByID = make(map[int]*Division)
	r.teamsByID = make(map[utils.UUID]*Team)
	r.individualsByID = make(map[utils.UUID]*Individual)
	r.gamesByID = make(map[utils.UUID]*Game)
	r.rostersByTeamID = make(map[utils.UUID]*Roster)
	r.individualStatusesByIndividualID = make(map[utils.UUID]*IndividualStatus)
	r.vendorToEntityID = make(map[vendorLookupKey]utils.UUID)
	r.entityToVendorID = make(map[entityVendorKey]string)
}

// =============================================================================
// League
// =============================================================================

// GetLeague returns a registered League by database ID, or nil if not found.
func (r *ModelRegistry) GetLeague(id int) *League {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.leaguesByID[id]
}

// RegisterLeague adds a League to the registry and returns a pointer to the registered instance.
// If a League with the same ID already exists, returns the existing pointer.
func (r *ModelRegistry) RegisterLeague(league *League) *League {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.leaguesByID[league.ID]; ok {
		return existing
	}

	r.leagues = append(r.leagues, *league)
	ptr := &r.leagues[len(r.leagues)-1]
	r.leaguesByID[league.ID] = ptr
	return ptr
}

// =============================================================================
// Conference
// =============================================================================

// GetConference returns a registered Conference by database ID, or nil if not found.
func (r *ModelRegistry) GetConference(id int) *Conference {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conferencesByID[id]
}

// RegisterConference adds a Conference to the registry and returns a pointer to the registered instance.
// If a Conference with the same ID already exists, returns the existing pointer.
func (r *ModelRegistry) RegisterConference(conference *Conference) *Conference {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.conferencesByID[conference.ID]; ok {
		return existing
	}

	r.conferences = append(r.conferences, *conference)
	ptr := &r.conferences[len(r.conferences)-1]
	r.conferencesByID[conference.ID] = ptr
	return ptr
}

// =============================================================================
// Division
// =============================================================================

// GetDivision returns a registered Division by database ID, or nil if not found.
func (r *ModelRegistry) GetDivision(id int) *Division {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.divisionsByID[id]
}

// RegisterDivision adds a Division to the registry and returns a pointer to the registered instance.
// If a Division with the same ID already exists, returns the existing pointer.
func (r *ModelRegistry) RegisterDivision(division *Division) *Division {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.divisionsByID[division.ID]; ok {
		return existing
	}

	r.divisions = append(r.divisions, *division)
	ptr := &r.divisions[len(r.divisions)-1]
	r.divisionsByID[division.ID] = ptr
	return ptr
}

// =============================================================================
// Team
// =============================================================================

// GetTeam returns a registered Team by database UUID, or nil if not found.
func (r *ModelRegistry) GetTeam(id utils.UUID) *Team {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.teamsByID[id]
}

// RegisterTeam adds a Team to the registry and returns a pointer to the registered instance.
// If a Team with the same ID already exists, returns the existing pointer.
// Returns an error if SportradarID is empty.
func (r *ModelRegistry) RegisterTeam(team *Team) (*Team, error) {
	if team.SportradarID == "" {
		return nil, fmt.Errorf("cannot register team %s: empty SportradarID", team.ID.String())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.teamsByID[team.ID]; ok {
		return existing, nil
	}

	r.teams = append(r.teams, *team)
	ptr := &r.teams[len(r.teams)-1]
	r.teamsByID[team.ID] = ptr
	r.registerVendorIDLocked(gen.EntityTeam, team.ID, gen.VendorSportradar, team.SportradarID)
	return ptr, nil
}

// =============================================================================
// Individual
// =============================================================================

// GetIndividual returns a registered Individual by database UUID, or nil if not found.
func (r *ModelRegistry) GetIndividual(id utils.UUID) *Individual {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.individualsByID[id]
}

// RegisterIndividual adds an Individual to the registry and returns a pointer to the registered instance.
// If an Individual with the same ID already exists, returns the existing pointer.
// Returns an error if SportradarID is empty.
func (r *ModelRegistry) RegisterIndividual(individual *Individual) (*Individual, error) {
	if individual.SportradarID == "" {
		return nil, fmt.Errorf("cannot register individual %s: empty SportradarID", individual.ID.String())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.individualsByID[individual.ID]; ok {
		return existing, nil
	}

	r.individuals = append(r.individuals, *individual)
	ptr := &r.individuals[len(r.individuals)-1]
	r.individualsByID[individual.ID] = ptr
	r.registerVendorIDLocked(gen.EntityIndividual, individual.ID, gen.VendorSportradar, individual.SportradarID)
	return ptr, nil
}

// =============================================================================
// Game
// =============================================================================

// GetGame returns a registered Game by database UUID, or nil if not found.
func (r *ModelRegistry) GetGame(id utils.UUID) *Game {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gamesByID[id]
}

// RegisterGame adds a Game to the registry and returns a pointer to the registered instance.
// If a Game with the same ID already exists, returns the existing pointer.
// Returns an error if SportradarID is empty.
func (r *ModelRegistry) RegisterGame(game *Game) (*Game, error) {
	if game.SportradarID == "" {
		return nil, fmt.Errorf("cannot register game %s: empty SportradarID", game.ID.String())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.gamesByID[game.ID]; ok {
		return existing, nil
	}

	r.games = append(r.games, *game)
	ptr := &r.games[len(r.games)-1]
	r.gamesByID[game.ID] = ptr
	r.registerVendorIDLocked(gen.EntityGame, game.ID, gen.VendorSportradar, game.SportradarID)
	return ptr, nil
}

// =============================================================================
// Roster
// =============================================================================

// GetRoster returns a registered Roster by team UUID, or nil if not found.
func (r *ModelRegistry) GetRoster(teamID utils.UUID) *Roster {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rostersByTeamID[teamID]
}

// RegisterRoster adds a Roster to the registry and returns a pointer to the registered instance.
// If a Roster with the same team ID already exists, returns the existing pointer.
func (r *ModelRegistry) RegisterRoster(roster *Roster) *Roster {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.rostersByTeamID[roster.TeamID]; ok {
		return existing
	}

	r.rosters = append(r.rosters, *roster)
	ptr := &r.rosters[len(r.rosters)-1]
	r.rostersByTeamID[roster.TeamID] = ptr
	return ptr
}

// =============================================================================
// IndividualStatus
// =============================================================================

// GetIndividualStatus returns a registered IndividualStatus by individual UUID, or nil if not found.
func (r *ModelRegistry) GetIndividualStatus(individualID utils.UUID) *IndividualStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.individualStatusesByIndividualID[individualID]
}

// RegisterIndividualStatus adds an IndividualStatus to the registry and returns a pointer to the registered instance.
// If an IndividualStatus with the same individual ID already exists, returns the existing pointer.
func (r *ModelRegistry) RegisterIndividualStatus(status *IndividualStatus) *IndividualStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.individualStatusesByIndividualID[status.IndividualID]; ok {
		return existing
	}

	r.individualStatuses = append(r.individualStatuses, *status)
	ptr := &r.individualStatuses[len(r.individualStatuses)-1]
	r.individualStatusesByIndividualID[status.IndividualID] = ptr
	return ptr
}

// =============================================================================
// Vendor IDs
// =============================================================================

// registerVendorIDLocked adds a vendor ID mapping. Must be called with mu write-lock held.
func (r *ModelRegistry) registerVendorIDLocked(entityType gen.Entity, entityID utils.UUID, vendor gen.Vendor, vendorID string) {
	r.vendorToEntityID[vendorLookupKey{EntityType: entityType, Vendor: vendor, VendorID: vendorID}] = entityID
	r.entityToVendorID[entityVendorKey{EntityType: entityType, EntityID: entityID, Vendor: vendor}] = vendorID
}

// RegisterVendorID adds a vendor ID mapping for an entity.
func (r *ModelRegistry) RegisterVendorID(entityType gen.Entity, entityID utils.UUID, vendor gen.Vendor, vendorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerVendorIDLocked(entityType, entityID, vendor, vendorID)
}

// GetEntityIDByVendorID returns the entity UUID for a given vendor ID, or zero UUID and false if not found.
func (r *ModelRegistry) GetEntityIDByVendorID(entityType gen.Entity, vendor gen.Vendor, vendorID string) (utils.UUID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.vendorToEntityID[vendorLookupKey{EntityType: entityType, Vendor: vendor, VendorID: vendorID}]
	return id, ok
}

// GetVendorID returns the vendor ID for a given entity, or empty string and false if not found.
func (r *ModelRegistry) GetVendorID(entityType gen.Entity, entityID utils.UUID, vendor gen.Vendor) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	vid, ok := r.entityToVendorID[entityVendorKey{EntityType: entityType, EntityID: entityID, Vendor: vendor}]
	return vid, ok
}

// GetTeamByVendorID returns a registered Team by vendor ID, or nil if not found.
func (r *ModelRegistry) GetTeamByVendorID(vendor gen.Vendor, vendorID string) *Team {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entityID, ok := r.vendorToEntityID[vendorLookupKey{EntityType: gen.EntityTeam, Vendor: vendor, VendorID: vendorID}]
	if !ok {
		return nil
	}
	return r.teamsByID[entityID]
}

// GetIndividualByVendorID returns a registered Individual by vendor ID, or nil if not found.
func (r *ModelRegistry) GetIndividualByVendorID(vendor gen.Vendor, vendorID string) *Individual {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entityID, ok := r.vendorToEntityID[vendorLookupKey{EntityType: gen.EntityIndividual, Vendor: vendor, VendorID: vendorID}]
	if !ok {
		return nil
	}
	return r.individualsByID[entityID]
}

// GetGameByVendorID returns a registered Game by vendor ID, or nil if not found.
func (r *ModelRegistry) GetGameByVendorID(vendor gen.Vendor, vendorID string) *Game {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entityID, ok := r.vendorToEntityID[vendorLookupKey{EntityType: gen.EntityGame, Vendor: vendor, VendorID: vendorID}]
	if !ok {
		return nil
	}
	return r.gamesByID[entityID]
}

// =============================================================================
// String
// =============================================================================

// String returns a formatted string representation of all registered entities.
func (r *ModelRegistry) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder

	// Leagues
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString("LEAGUES\n")
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.leagues {
		sb.WriteString(r.leagues[i].String())
	}

	// Conferences
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString("CONFERENCES\n")
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.conferences {
		sb.WriteString(r.conferences[i].String())
	}

	// Divisions
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString("DIVISIONS\n")
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.divisions {
		sb.WriteString(r.divisions[i].String())
	}

	// Teams
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString("TEAMS\n")
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.teams {
		sb.WriteString(r.teams[i].String())
	}

	// Individuals (show first 10)
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString(fmt.Sprintf("INDIVIDUALS (showing first 10 of %d)\n", len(r.individuals)))
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.individuals {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("\n... and %d more individuals\n", len(r.individuals)-10))
			break
		}
		sb.WriteString(r.individuals[i].String())
	}

	// Rosters
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString("ROSTERS\n")
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.rosters {
		sb.WriteString(r.rosters[i].String())
	}

	// Games
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString(fmt.Sprintf("GAMES (showing all %d)\n", len(r.games)))
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.games {
		sb.WriteString(r.games[i].String())
	}

	// Individual Statuses
	sb.WriteString("\n" + strings.Repeat("=", 72) + "\n")
	sb.WriteString(fmt.Sprintf("INDIVIDUAL STATUSES (showing all %d)\n", len(r.individualStatuses)))
	sb.WriteString(strings.Repeat("=", 72) + "\n")
	for i := range r.individualStatuses {
		sb.WriteString(r.individualStatuses[i].String())
	}

	return sb.String()
}
