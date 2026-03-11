package tester_common

import (
	"fmt"
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
	OrderID           string
	ClientOrderID     uint64
	Portion           uint64
	OriginalQuantity  uint64
	RemainingQuantity uint64
	SequenceNumber    uint64
}

// LineupState contains all orders for a specific lineup
type LineupState struct {
	Orders map[string]*OrderState // map[order_id]OrderState
}

// PoolState represents a single entry pool
type PoolState struct {
	LegSecurityIDs []utils.UUID            // Sorted leg security IDs (pool key) - legs mode
	SlateID        string                  // Slate ID string - slate mode
	Lineups        map[uint64]*LineupState // map[lineup_index]LineupState
	NumLegs        int                     // Number of legs - legs mode
	NumLineups     uint64                  // Number of lineups - slate mode (from DefinePool)
	TotalUnits     uint64                  // Total units - slate mode (from DefinePool)
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

// DefinePool registers a pool by slate ID for slate-based tracking
func (pt *PoolTracker) DefinePool(slateID string, totalUnits uint64, numLineups uint64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.pools[slateID]; exists {
		return
	}

	pt.pools[slateID] = &PoolState{
		SlateID:    slateID,
		Lineups:    make(map[uint64]*LineupState),
		NumLineups: numLineups,
		TotalUnits: totalUnits,
	}
}

// PoolKeyToString converts sorted leg IDs to a string key
func PoolKeyToString(legIDs []utils.UUID) string {
	result := ""
	for i, id := range legIDs {
		if i > 0 {
			result += ","
		}
		result += id.String()
	}
	return result
}

// CompareUUIDs returns -1, 0, or 1 comparing two UUIDs lexicographically
func CompareUUIDs(a, b utils.UUID) int {
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

// CreateSortedLegIDs creates a sorted list of leg security IDs from legs
func CreateSortedLegIDs(legs []Leg) []utils.UUID {
	ids := make([]utils.UUID, len(legs))
	for i, leg := range legs {
		ids[i] = leg.LegSecurityID
	}
	sort.Slice(ids, func(i, j int) bool {
		return CompareUUIDs(ids[i], ids[j]) < 0
	})
	return ids
}

// CalculateLineupIndex calculates the lineup index from legs
// Based on the formula: lineup_index = sum(is_over[i] * 2^i for i in 0..n)
// where legs are sorted by leg_security_id
func CalculateLineupIndex(legs []Leg) uint64 {
	// Sort legs by security ID
	sortedLegs := make([]Leg, len(legs))
	copy(sortedLegs, legs)
	sort.Slice(sortedLegs, func(i, j int) bool {
		return CompareUUIDs(sortedLegs[i].LegSecurityID, sortedLegs[j].LegSecurityID) < 0
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
	sortedLegIDs := CreateSortedLegIDs(legs)
	poolKey := PoolKeyToString(sortedLegIDs)

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
	lineupIndex := CalculateLineupIndex(legs)

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

// AddOrderBySlateID adds an order using slate ID and lineup index (matching server mode)
func (pt *PoolTracker) AddOrderBySlateID(
	orderID string,
	clientOrderID uint64,
	slateID string,
	lineupIndex uint64,
	portion uint64,
	quantity uint64,
	sequenceNumber uint64,
) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pool, exists := pt.pools[slateID]
	if !exists {
		pool = &PoolState{
			SlateID:    slateID,
			Lineups:    make(map[uint64]*LineupState),
			NumLineups: lineupIndex + 1,
		}
		pt.pools[slateID] = pool
	}

	lineup, exists := pool.Lineups[lineupIndex]
	if !exists {
		lineup = &LineupState{
			Orders: make(map[string]*OrderState),
		}
		pool.Lineups[lineupIndex] = lineup
	}

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
	SlateID        string
	NumLegs        int
	NumLineups     uint64
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

// GetOrdersDisplay extracts and sorts orders from a lineup for display
func GetOrdersDisplay(lineup *LineupState) []OrderDisplayData {
	orders := make([]OrderDisplayData, 0)
	if lineup == nil {
		return orders
	}
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
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].Portion != orders[j].Portion {
			return orders[i].Portion > orders[j].Portion
		}
		return orders[i].SequenceNumber < orders[j].SequenceNumber
	})
	return orders
}

// ReplacePoolFromSnapshot replaces all pool state from an OrderPoolSnapshot.
// Each snapshot represents the complete current state, so all existing pools are
// cleared first. Since snapshots contain anonymous aggregate data (no order IDs),
// synthetic IDs are used.
func (pt *PoolTracker) ReplacePoolFromSnapshot(
	legSecurityIDs []utils.UUID,
	lineupBooks []LineupBookSnapshot,
) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Clear all existing pools since each snapshot represents the full state
	for key := range pt.pools {
		delete(pt.pools, key)
	}

	sortedIDs := make([]utils.UUID, len(legSecurityIDs))
	copy(sortedIDs, legSecurityIDs)
	sort.Slice(sortedIDs, func(i, j int) bool {
		return CompareUUIDs(sortedIDs[i], sortedIDs[j]) < 0
	})
	poolKey := PoolKeyToString(sortedIDs)

	pool := &PoolState{
		LegSecurityIDs: sortedIDs,
		Lineups:        make(map[uint64]*LineupState),
		NumLegs:        len(legSecurityIDs),
	}

	for lineupIdx, book := range lineupBooks {
		lineup := &LineupState{
			Orders: make(map[string]*OrderState),
		}

		orderCounter := uint64(0)
		for levelIdx, level := range book.Levels {
			for orderIdx, order := range level.Orders {
				orderCounter++
				syntheticID := fmt.Sprintf("snapshot-%d-%d-%d", lineupIdx, levelIdx, orderIdx)
				lineup.Orders[syntheticID] = &OrderState{
					OrderID:           syntheticID,
					Portion:           level.Portion,
					OriginalQuantity:  order.QuantityRemaining,
					RemainingQuantity: order.QuantityRemaining,
					SequenceNumber:    orderCounter, // Stable ordering within lineup
				}
			}
		}

		pool.Lineups[uint64(lineupIdx)] = lineup
	}

	pt.pools[poolKey] = pool
}

// LineupBookSnapshot represents a lineup from an OrderPoolSnapshot
type LineupBookSnapshot struct {
	IsOver []bool
	Levels []LevelSnapshot
}

// LevelSnapshot represents a price level from an OrderPoolSnapshot
type LevelSnapshot struct {
	Portion uint64
	Orders  []OrderSnapshot
}

// OrderSnapshot represents an order from an OrderPoolSnapshot
type OrderSnapshot struct {
	QuantityRemaining uint64
}

// GetAllPoolsDisplay returns all pools formatted for display
func (pt *PoolTracker) GetAllPoolsDisplay() []PoolDisplayData {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]PoolDisplayData, 0, len(pt.pools))

	for poolKey, pool := range pt.pools {
		if pool.SlateID != "" {
			// Slate-based pool
			lineups := make([]LineupDisplayData, 0, pool.NumLineups)
			for lineupIdx := uint64(0); lineupIdx < pool.NumLineups; lineupIdx++ {
				lineups = append(lineups, LineupDisplayData{
					LineupIndex: lineupIdx,
					Orders:      GetOrdersDisplay(pool.Lineups[lineupIdx]),
				})
			}

			result = append(result, PoolDisplayData{
				PoolKey:    poolKey,
				SlateID:    pool.SlateID,
				NumLineups: pool.NumLineups,
				TotalUnits: pool.TotalUnits,
				Lineups:    lineups,
			})
		} else {
			// Legs-based pool
			numLineups := uint64(1 << uint(pool.NumLegs))
			lineups := make([]LineupDisplayData, 0, numLineups)

			for lineupIdx := uint64(0); lineupIdx < numLineups; lineupIdx++ {
				overUnders := make([]OverUnderDisplay, pool.NumLegs)
				for i := 0; i < pool.NumLegs; i++ {
					isOver := (lineupIdx & (1 << uint(i))) != 0
					overUnders[i] = OverUnderDisplay{
						LegSecurityID: pool.LegSecurityIDs[i].String(),
						IsOver:        isOver,
					}
				}

				lineups = append(lineups, LineupDisplayData{
					LineupIndex: lineupIdx,
					OverUnders:  overUnders,
					Orders:      GetOrdersDisplay(pool.Lineups[lineupIdx]),
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
	}

	// Sort pools by pool key for consistent display
	sort.Slice(result, func(i, j int) bool {
		return result[i].PoolKey < result[j].PoolKey
	})

	return result
}

// PendingOrderInfo stores pending order data in a mode-agnostic way for pool tracking
type PendingOrderInfo struct {
	Legs        []Leg
	SlateID     string // matching server mode: slate ID for pool tracker
	LineupIndex uint64 // matching server mode: lineup index for pool tracker
	Portion     uint64
	Quantity    uint64
}
