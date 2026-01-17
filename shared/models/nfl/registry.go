package nfl

import "sync"

// NFLRegistry provides thread-safe storage and retrieval of NFL-specific model instances.
// See models.ModelRegistry for the general pattern documentation.
type NFLRegistry struct {
	mu sync.RWMutex

	// Slices own the data
	nflStats []NFLStats

	// Maps for O(1) lookups by database ID
	nflStatsByID map[int]*NFLStats
}

// Registry is the global singleton for NFL model instances.
var Registry = NewNFLRegistry()

// NewNFLRegistry creates a new initialized NFLRegistry.
func NewNFLRegistry() *NFLRegistry {
	return &NFLRegistry{
		nflStatsByID: make(map[int]*NFLStats),
	}
}

// Clear resets all registry data.
func (r *NFLRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nflStats = nil
	r.nflStatsByID = make(map[int]*NFLStats)
}

// GetNFLStats returns a registered NFLStats by database ID, or nil if not found.
func (r *NFLRegistry) GetNFLStats(id int) *NFLStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nflStatsByID[id]
}

// RegisterNFLStats adds NFLStats to the registry and returns a pointer to the registered instance.
// If NFLStats with the same ID already exists, returns the existing pointer.
func (r *NFLRegistry) RegisterNFLStats(stats *NFLStats) *NFLStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.nflStatsByID[stats.ID]; ok {
		return existing
	}

	r.nflStats = append(r.nflStats, *stats)
	ptr := &r.nflStats[len(r.nflStats)-1]
	r.nflStatsByID[stats.ID] = ptr
	return ptr
}
