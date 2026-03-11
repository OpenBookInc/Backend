package nfl

import (
	"sync"

	"github.com/openbook/shared/utils"
)

// boxScoreKey is a composite key for NFL box scores (game_id, individual_id).
type boxScoreKey struct {
	GameID       utils.UUID
	IndividualID utils.UUID
}

// NFLRegistry provides thread-safe storage and retrieval of NFL-specific model instances.
// See models.ModelRegistry for the general pattern documentation.
type NFLRegistry struct {
	mu sync.RWMutex

	// Slices own the data
	nflStats []NFLStats

	// Maps for O(1) lookups by composite key (game_id, individual_id)
	nflStatsByKey map[boxScoreKey]*NFLStats
}

// Registry is the global singleton for NFL model instances.
var Registry = NewNFLRegistry()

// NewNFLRegistry creates a new initialized NFLRegistry.
func NewNFLRegistry() *NFLRegistry {
	return &NFLRegistry{
		nflStatsByKey: make(map[boxScoreKey]*NFLStats),
	}
}

// Clear resets all registry data.
func (r *NFLRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nflStats = nil
	r.nflStatsByKey = make(map[boxScoreKey]*NFLStats)
}

// GetNFLStats returns a registered NFLStats by game and individual ID, or nil if not found.
func (r *NFLRegistry) GetNFLStats(gameID, individualID utils.UUID) *NFLStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nflStatsByKey[boxScoreKey{GameID: gameID, IndividualID: individualID}]
}

// RegisterNFLStats adds NFLStats to the registry and returns a pointer to the registered instance.
// If NFLStats with the same composite key already exists, returns the existing pointer.
func (r *NFLRegistry) RegisterNFLStats(stats *NFLStats) *NFLStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := boxScoreKey{GameID: stats.Game.ID, IndividualID: stats.Individual.ID}
	if existing, ok := r.nflStatsByKey[key]; ok {
		return existing
	}

	r.nflStats = append(r.nflStats, *stats)
	ptr := &r.nflStats[len(r.nflStats)-1]
	r.nflStatsByKey[key] = ptr
	return ptr
}
