package nba

import "sync"

// NBARegistry provides thread-safe storage and retrieval of NBA-specific model instances.
// See models.ModelRegistry for the general pattern documentation.
type NBARegistry struct {
	mu sync.RWMutex

	// Slices own the data
	nbaStats []NBAStats

	// Maps for O(1) lookups by database ID
	nbaStatsByID map[int]*NBAStats
}

// Registry is the global singleton for NBA model instances.
var Registry = NewNBARegistry()

// NewNBARegistry creates a new initialized NBARegistry.
func NewNBARegistry() *NBARegistry {
	return &NBARegistry{
		nbaStatsByID: make(map[int]*NBAStats),
	}
}

// Clear resets all registry data.
func (r *NBARegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nbaStats = nil
	r.nbaStatsByID = make(map[int]*NBAStats)
}

// GetNBAStats returns a registered NBAStats by database ID, or nil if not found.
func (r *NBARegistry) GetNBAStats(id int) *NBAStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nbaStatsByID[id]
}

// RegisterNBAStats adds NBAStats to the registry and returns a pointer to the registered instance.
// If NBAStats with the same ID already exists, returns the existing pointer.
func (r *NBARegistry) RegisterNBAStats(stats *NBAStats) *NBAStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.nbaStatsByID[stats.ID]; ok {
		return existing
	}

	r.nbaStats = append(r.nbaStats, *stats)
	ptr := &r.nbaStats[len(r.nbaStats)-1]
	r.nbaStatsByID[stats.ID] = ptr
	return ptr
}
