package nba

import (
	"sync"

	"github.com/openbook/shared/utils"
)

// boxScoreKey is a composite key for NBA box scores (game_id, individual_id).
type boxScoreKey struct {
	GameID       utils.UUID
	IndividualID utils.UUID
}

// NBARegistry provides thread-safe storage and retrieval of NBA-specific model instances.
// See models.ModelRegistry for the general pattern documentation.
type NBARegistry struct {
	mu sync.RWMutex

	// Slices own the data
	nbaStats []NBAStats

	// Maps for O(1) lookups by composite key (game_id, individual_id)
	nbaStatsByKey map[boxScoreKey]*NBAStats
}

// Registry is the global singleton for NBA model instances.
var Registry = NewNBARegistry()

// NewNBARegistry creates a new initialized NBARegistry.
func NewNBARegistry() *NBARegistry {
	return &NBARegistry{
		nbaStatsByKey: make(map[boxScoreKey]*NBAStats),
	}
}

// Clear resets all registry data.
func (r *NBARegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nbaStats = nil
	r.nbaStatsByKey = make(map[boxScoreKey]*NBAStats)
}

// GetNBAStats returns a registered NBAStats by game and individual ID, or nil if not found.
func (r *NBARegistry) GetNBAStats(gameID, individualID utils.UUID) *NBAStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nbaStatsByKey[boxScoreKey{GameID: gameID, IndividualID: individualID}]
}

// RegisterNBAStats adds NBAStats to the registry and returns a pointer to the registered instance.
// If NBAStats with the same composite key already exists, returns the existing pointer.
func (r *NBARegistry) RegisterNBAStats(stats *NBAStats) *NBAStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := boxScoreKey{GameID: stats.Game.ID, IndividualID: stats.Individual.ID}
	if existing, ok := r.nbaStatsByKey[key]; ok {
		return existing
	}

	r.nbaStats = append(r.nbaStats, *stats)
	ptr := &r.nbaStats[len(r.nbaStats)-1]
	r.nbaStatsByKey[key] = ptr
	return ptr
}
