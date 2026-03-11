package main

import (
	"sort"
	"sync"

	common "matching-clients/src/gen"
	gwpb "matching-clients/src/gen/gateway"
	"matching-clients/src/utils"
)

// TrackedLeg represents a single leg with its security ID and side.
type TrackedLeg struct {
	LegSecurityID utils.UUID
	IsOver        bool
}

// TrackedOrder holds the pool-relevant state of a confirmed order.
type TrackedOrder struct {
	DBRecordID           utils.UUID
	SlateID              utils.UUID
	Legs                 []TrackedLeg
	Portion              uint64
	RemainingQty         uint64
	OrderType            common.OrderType
	UserID               utils.UUID
	LineupIndex          uint64
	LocalEventSequence   uint64 // monotonic insertion sequence for FIFO ordering during resubmission
	BackendClientOrderID uint64 // uint64 client_order_id from the backend client
}

// OrderPoolTracker tracks all active orders grouped by slate for snapshot generation.
type OrderPoolTracker struct {
	mu     sync.RWMutex
	orders map[utils.UUID]*TrackedOrder // keyed by dbRecordID
}

// NewOrderPoolTracker creates a new OrderPoolTracker.
func NewOrderPoolTracker() *OrderPoolTracker {
	return &OrderPoolTracker{
		orders: make(map[utils.UUID]*TrackedOrder),
	}
}

// GetAllOrders returns a snapshot of all tracked orders.
func (pt *OrderPoolTracker) GetAllOrders() []*TrackedOrder {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	orders := make([]*TrackedOrder, 0, len(pt.orders))
	for _, order := range pt.orders {
		orders = append(orders, order)
	}
	return orders
}

// AddOrder adds a confirmed order to the pool tracker.
func (pt *OrderPoolTracker) AddOrder(order *TrackedOrder) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.orders[order.DBRecordID] = order
}

// RemoveOrderAndGetSlate removes an order and returns its slate ID for broadcast.
// Returns zero UUID if the order was not found.
func (pt *OrderPoolTracker) RemoveOrderAndGetSlate(dbRecordID utils.UUID) utils.UUID {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	order, exists := pt.orders[dbRecordID]
	if !exists {
		return utils.UUID{}
	}
	slateID := order.SlateID
	delete(pt.orders, dbRecordID)
	return slateID
}

// UpdateFillAndGetSlate reduces an order's remaining quantity, optionally removes it,
// and returns the slate ID for broadcast. Returns zero UUID if the order was not found.
func (pt *OrderPoolTracker) UpdateFillAndGetSlate(dbRecordID utils.UUID, matchedQuantity uint64, isComplete bool) utils.UUID {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	order, exists := pt.orders[dbRecordID]
	if !exists {
		return utils.UUID{}
	}

	slateID := order.SlateID

	if order.RemainingQty >= matchedQuantity {
		order.RemainingQty -= matchedQuantity
	} else {
		order.RemainingQty = 0
	}

	if isComplete {
		delete(pt.orders, dbRecordID)
	}
	return slateID
}

// slateGroup groups orders belonging to the same slate.
type slateGroup struct {
	slateID        utils.UUID
	legSecurityIDs []utils.UUID // sorted, from the first order's legs
	numLegs        int
	orders         []*TrackedOrder
}

// BuildSnapshotsForAllSlates builds one OrderPoolSnapshot per slate that has active orders.
func (pt *OrderPoolTracker) BuildSnapshotsForAllSlates() []*gwpb.OrderPoolSnapshot {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	// Group orders by slateID
	groups := make(map[utils.UUID]*slateGroup)
	for _, order := range pt.orders {
		g, exists := groups[order.SlateID]
		if !exists {
			g = &slateGroup{
				slateID: order.SlateID,
				numLegs: len(order.Legs),
			}
			// Derive sorted leg security IDs from this order's legs
			ids := make([]utils.UUID, len(order.Legs))
			for i, leg := range order.Legs {
				ids[i] = leg.LegSecurityID
			}
			sort.Slice(ids, func(i, j int) bool {
				return compareUUIDs(ids[i], ids[j]) < 0
			})
			g.legSecurityIDs = ids
			groups[order.SlateID] = g
		}
		g.orders = append(g.orders, order)
	}

	snapshots := make([]*gwpb.OrderPoolSnapshot, 0, len(groups))
	for _, g := range groups {
		snapshots = append(snapshots, buildSnapshot(g))
	}
	return snapshots
}

// BuildSnapshotsForSlate builds a snapshot for a single slate. Returns nil if the slate has no orders.
func (pt *OrderPoolTracker) BuildSnapshotsForSlate(slateID utils.UUID) *gwpb.OrderPoolSnapshot {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	g := &slateGroup{
		slateID: slateID,
	}

	for _, order := range pt.orders {
		if order.SlateID != slateID {
			continue
		}
		if g.numLegs == 0 {
			g.numLegs = len(order.Legs)
			ids := make([]utils.UUID, len(order.Legs))
			for i, leg := range order.Legs {
				ids[i] = leg.LegSecurityID
			}
			sort.Slice(ids, func(i, j int) bool {
				return compareUUIDs(ids[i], ids[j]) < 0
			})
			g.legSecurityIDs = ids
		}
		g.orders = append(g.orders, order)
	}

	if len(g.orders) == 0 {
		return nil
	}
	return buildSnapshot(g)
}

// buildSnapshot constructs a proto OrderPoolSnapshot from a slateGroup.
func buildSnapshot(g *slateGroup) *gwpb.OrderPoolSnapshot {
	// Calculate lineup index for each order and group by lineup
	type lineupOrders struct {
		isOvers []bool
		orders  []*TrackedOrder
	}
	lineups := make(map[uint64]*lineupOrders)

	for _, order := range g.orders {
		idx := calculateLineupIndex(order.Legs, g.legSecurityIDs)
		lu, exists := lineups[idx]
		if !exists {
			lu = &lineupOrders{
				isOvers: calculateIsOvers(order.Legs, g.legSecurityIDs),
			}
			lineups[idx] = lu
		}
		lu.orders = append(lu.orders, order)
	}

	// Build all lineup books (including empty lineups for completeness)
	numLineups := uint64(1) << uint(g.numLegs)
	lineupBooks := make([]*gwpb.OrderPoolSnapshot_LineupBook, numLineups)

	for lineupIdx := uint64(0); lineupIdx < numLineups; lineupIdx++ {
		isOvers := make([]bool, g.numLegs)
		for i := 0; i < g.numLegs; i++ {
			isOvers[i] = (lineupIdx & (1 << uint(i))) != 0
		}

		var levels []*gwpb.OrderPoolSnapshot_LineupBook_Level
		if lu, exists := lineups[lineupIdx]; exists {
			levels = buildLevels(lu.orders)
		}

		lineupBooks[lineupIdx] = &gwpb.OrderPoolSnapshot_LineupBook{
			IsOver: isOvers,
			Levels: levels,
		}
	}

	// Build leg security ID protos
	legIDs := make([]*common.UUID, len(g.legSecurityIDs))
	for i, id := range g.legSecurityIDs {
		legIDs[i] = &common.UUID{Upper: id.Upper(), Lower: id.Lower()}
	}

	return &gwpb.OrderPoolSnapshot{
		SlateId:        &common.UUID{Upper: g.slateID.Upper(), Lower: g.slateID.Lower()},
		LegSecurityIds: legIDs,
		LineupBooks:    lineupBooks,
	}
}

// buildLevels groups orders by portion into price levels, sorted by portion descending.
func buildLevels(orders []*TrackedOrder) []*gwpb.OrderPoolSnapshot_LineupBook_Level {
	// Group by portion
	portionOrders := make(map[uint64][]*TrackedOrder)
	for _, o := range orders {
		portionOrders[o.Portion] = append(portionOrders[o.Portion], o)
	}

	// Sort portions descending
	portions := make([]uint64, 0, len(portionOrders))
	for p := range portionOrders {
		portions = append(portions, p)
	}
	sort.Slice(portions, func(i, j int) bool {
		return portions[i] > portions[j]
	})

	levels := make([]*gwpb.OrderPoolSnapshot_LineupBook_Level, len(portions))
	for i, portion := range portions {
		protoOrders := make([]*gwpb.OrderPoolSnapshot_LineupBook_Level_Order, len(portionOrders[portion]))
		for j, o := range portionOrders[portion] {
			protoOrders[j] = &gwpb.OrderPoolSnapshot_LineupBook_Level_Order{
				QuantityRemaining: o.RemainingQty,
				OrderId:           &common.UUID{Upper: o.DBRecordID.Upper(), Lower: o.DBRecordID.Lower()},
				ClientOrderId:     o.BackendClientOrderID,
				UserId:            &common.UUID{Upper: o.UserID.Upper(), Lower: o.UserID.Lower()},
			}
		}
		levels[i] = &gwpb.OrderPoolSnapshot_LineupBook_Level{
			Portion: portion,
			Orders:  protoOrders,
		}
	}
	return levels
}

// calculateLineupIndex computes the lineup index for an order's legs relative to sorted leg security IDs.
// The lineup index is a bitmask where bit i is set if the leg at sorted position i is over.
func calculateLineupIndex(legs []TrackedLeg, sortedLegIDs []utils.UUID) uint64 {
	// Build map: legSecurityID → isOver
	sideMap := make(map[utils.UUID]bool, len(legs))
	for _, leg := range legs {
		sideMap[leg.LegSecurityID] = leg.IsOver
	}

	var idx uint64
	for i, id := range sortedLegIDs {
		if sideMap[id] {
			idx |= 1 << uint(i)
		}
	}
	return idx
}

// calculateIsOvers returns the is_over flags in sorted leg security ID order.
func calculateIsOvers(legs []TrackedLeg, sortedLegIDs []utils.UUID) []bool {
	sideMap := make(map[utils.UUID]bool, len(legs))
	for _, leg := range legs {
		sideMap[leg.LegSecurityID] = leg.IsOver
	}

	result := make([]bool, len(sortedLegIDs))
	for i, id := range sortedLegIDs {
		result[i] = sideMap[id]
	}
	return result
}

// compareUUIDs returns -1, 0, or 1 comparing two UUIDs lexicographically.
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
