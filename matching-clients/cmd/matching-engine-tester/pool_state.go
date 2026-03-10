package main

import (
	"matching-clients/src/utils"
	"sort"
	"sync"
)

// Leg represents a single leg in an order
type Leg struct {
	LegSecurityID utils.UUID
	IsOver        bool
}

// OrderState tracks the current state of an order
type OrderState struct {
	OrderID          string
	ClientOrderID    uint64
	Portion          uint64
	OriginalQuantity uint64
	RemainingQuantity uint64
	SequenceNumber   uint64
}

// LineupState contains all orders for a specific lineup
type LineupState struct {
	Orders map[string]*OrderState // map[order_id]OrderState
}

// PoolState represents a single entry pool
type PoolState struct {
	LegSecurityIDs []utils.UUID              // Sorted leg security IDs (pool key)
	Lineups        map[uint64]*LineupState   // map[lineup_index]LineupState
	NumLegs        int
}

// PoolTracker manages all entry pools
type PoolTracker struct {
	mu    sync.RWMutex
	pools map[string]*PoolState // map[pool_key_string]PoolState
}

// NewPoolTracker creates a new pool tracker
func NewPoolTracker() *PoolTracker {
	return &PoolTracker{
		pools: make(map[string]*PoolState),
	}
}

// poolKeyToString converts sorted leg IDs to a string key
func poolKeyToString(legIDs []utils.UUID) string {
	result := ""
	for i, id := range legIDs {
		if i > 0 {
			result += ","
		}
		result += id.String()
	}
	return result
}

// compareUUIDs returns -1, 0, or 1 comparing two UUIDs lexicographically
func compareUUIDs(a, b utils.UUID) int {
	if a.Upper() < b.Upper() {
		return -1
	}
	if a.Upper() > b.Upper() {
		return 1
	}
	if a.Lower() < b.Lower() {
		return -1
	}
	if a.Lower() > b.Lower() {
		return 1
	}
	return 0
}

// createSortedLegIDs creates a sorted list of leg security IDs from legs
func createSortedLegIDs(legs []Leg) []utils.UUID {
	ids := make([]utils.UUID, len(legs))
	for i, leg := range legs {
		ids[i] = leg.LegSecurityID
	}
	sort.Slice(ids, func(i, j int) bool {
		return compareUUIDs(ids[i], ids[j]) < 0
	})
	return ids
}

// calculateLineupIndex calculates the lineup index from legs
// Based on the formula: lineup_index = sum(is_over[i] * 2^i for i in 0..n)
// where legs are sorted by leg_security_id
func calculateLineupIndex(legs []Leg) uint64 {
	// Sort legs by security ID
	sortedLegs := make([]Leg, len(legs))
	copy(sortedLegs, legs)
	sort.Slice(sortedLegs, func(i, j int) bool {
		return compareUUIDs(sortedLegs[i].LegSecurityID, sortedLegs[j].LegSecurityID) < 0
	})

	// Calculate lineup index
	var lineupIndex uint64 = 0
	for i, leg := range sortedLegs {
		if leg.IsOver {
			lineupIndex |= (1 << uint(i))
		}
	}
	return lineupIndex
}

// AddOrder adds a new order to the tracker
func (pt *PoolTracker) AddOrder(
	orderID string,
	clientOrderID uint64,
	legs []Leg,
	portion uint64,
	quantity uint64,
	sequenceNumber uint64,
) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Create pool key
	sortedLegIDs := createSortedLegIDs(legs)
	poolKey := poolKeyToString(sortedLegIDs)

	// Get or create pool
	pool, exists := pt.pools[poolKey]
	if !exists {
		pool = &PoolState{
			LegSecurityIDs: sortedLegIDs,
			Lineups:        make(map[uint64]*LineupState),
			NumLegs:        len(legs),
		}
		pt.pools[poolKey] = pool
	}

	// Calculate lineup index
	lineupIndex := calculateLineupIndex(legs)

	// Get or create lineup
	lineup, exists := pool.Lineups[lineupIndex]
	if !exists {
		lineup = &LineupState{
			Orders: make(map[string]*OrderState),
		}
		pool.Lineups[lineupIndex] = lineup
	}

	// Add order
	lineup.Orders[orderID] = &OrderState{
		OrderID:           orderID,
		ClientOrderID:     clientOrderID,
		Portion:           portion,
		OriginalQuantity:  quantity,
		RemainingQuantity: quantity,
		SequenceNumber:    sequenceNumber,
	}
}

// UpdateFromFill updates order quantities based on a fill event
func (pt *PoolTracker) UpdateFromFill(orderID string, matchedQuantity uint64, isComplete bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Find the order across all pools and lineups
	for _, pool := range pt.pools {
		for _, lineup := range pool.Lineups {
			if order, exists := lineup.Orders[orderID]; exists {
				// Reduce remaining quantity
				if order.RemainingQuantity >= matchedQuantity {
					order.RemainingQuantity -= matchedQuantity
				} else {
					order.RemainingQuantity = 0
				}

				// Remove if complete
				if isComplete {
					delete(lineup.Orders, orderID)
				}
				return
			}
		}
	}
}

// RemoveOrder removes an order (e.g., due to cancellation)
func (pt *PoolTracker) RemoveOrder(orderID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Find and remove the order across all pools and lineups
	for _, pool := range pt.pools {
		for _, lineup := range pool.Lineups {
			if _, exists := lineup.Orders[orderID]; exists {
				delete(lineup.Orders, orderID)
				return
			}
		}
	}
}

// PoolDisplayData represents data for displaying a pool
type PoolDisplayData struct {
	PoolKey        string
	LegSecurityIDs []string
	NumLegs        int
	TotalUnits     uint64
	Lineups        []LineupDisplayData
}

// LineupDisplayData represents data for displaying a lineup
type LineupDisplayData struct {
	LineupIndex uint64
	OverUnders  []OverUnderDisplay
	Orders      []OrderDisplayData
}

// OverUnderDisplay shows the over/under status for a leg
type OverUnderDisplay struct {
	LegSecurityID string
	IsOver        bool
}

// OrderDisplayData represents data for displaying an order
type OrderDisplayData struct {
	OrderID           string
	ClientOrderID     uint64
	Portion           uint64
	OriginalQuantity  uint64
	RemainingQuantity uint64
	SequenceNumber    uint64
}

// GetAllPoolsDisplay returns all pools formatted for display
func (pt *PoolTracker) GetAllPoolsDisplay() []PoolDisplayData {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]PoolDisplayData, 0, len(pt.pools))

	for poolKey, pool := range pt.pools {
		// Calculate number of possible lineups (2^num_legs)
		numLineups := uint64(1 << uint(pool.NumLegs))

		// Create lineup display data for all possible lineups (even empty ones)
		lineups := make([]LineupDisplayData, 0, numLineups)

		for lineupIdx := uint64(0); lineupIdx < numLineups; lineupIdx++ {
			// Calculate over/under combination for this lineup
			overUnders := make([]OverUnderDisplay, pool.NumLegs)
			for i := 0; i < pool.NumLegs; i++ {
				isOver := (lineupIdx & (1 << uint(i))) != 0
				overUnders[i] = OverUnderDisplay{
					LegSecurityID: pool.LegSecurityIDs[i].String(),
					IsOver:        isOver,
				}
			}

			// Get orders for this lineup
			orders := make([]OrderDisplayData, 0)
			if lineup, exists := pool.Lineups[lineupIdx]; exists {
				for _, order := range lineup.Orders {
					orders = append(orders, OrderDisplayData{
						OrderID:           order.OrderID,
						ClientOrderID:     order.ClientOrderID,
						Portion:           order.Portion,
						OriginalQuantity:  order.OriginalQuantity,
						RemainingQuantity: order.RemainingQuantity,
						SequenceNumber:    order.SequenceNumber,
					})
				}
			}

			// Sort orders: highest portion first, then by sequence number (FIFO)
			sort.Slice(orders, func(i, j int) bool {
				if orders[i].Portion != orders[j].Portion {
					return orders[i].Portion > orders[j].Portion
				}
				return orders[i].SequenceNumber < orders[j].SequenceNumber
			})

			lineups = append(lineups, LineupDisplayData{
				LineupIndex: lineupIdx,
				OverUnders:  overUnders,
				Orders:      orders,
			})
		}

		legIDStrings := make([]string, len(pool.LegSecurityIDs))
		for i, id := range pool.LegSecurityIDs {
			legIDStrings[i] = id.String()
		}

		result = append(result, PoolDisplayData{
			PoolKey:        poolKey,
			LegSecurityIDs: legIDStrings,
			NumLegs:        pool.NumLegs,
			TotalUnits:     1_000_000,
			Lineups:        lineups,
		})
	}

	// Sort pools by pool key for consistent display
	sort.Slice(result, func(i, j int) bool {
		return result[i].PoolKey < result[j].PoolKey
	})

	return result
}
