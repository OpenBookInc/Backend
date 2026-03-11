package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	common "matching-clients/src/gen"
	pb "matching-clients/src/gen/matching"
	"github.com/openbook/shared/utils"

	gen "github.com/openbook/shared/models/gen"
)

// PendingCancel tracks a cancel request that is waiting for its order to be acked
// so the engine order ID is known and the CancelOrder can be sent.
type PendingCancel struct {
	DBRecordID         utils.UUID
	IsRecovery         bool
	LocalEventSequence uint64
}

// ActiveExchangeState represents the full active state recovered from the database.
type ActiveExchangeState struct {
	Orders         []ActiveExchangeOrder  `json:"orders"`
	CancelRequests []ActiveCancelRequest  `json:"cancel_requests"`
}

// ActiveExchangeOrder represents an active order recovered from the database.
type ActiveExchangeOrder struct {
	OrderID           utils.UUID       `json:"order_id"`
	UserID            utils.UUID       `json:"user_id"`
	OrderType         string           `json:"order_type"`
	Portion           int64            `json:"portion"`
	Quantity          int64            `json:"quantity"`
	RemainingQuantity int64            `json:"remaining_quantity"`
	SlateID           utils.UUID       `json:"slate_id"`
	TotalUnits        int64            `json:"total_units"`
	LineupIndex       int              `json:"lineup_index"`
	Legs              []ActiveOrderLeg `json:"legs"`
	CreatedAt         string           `json:"created_at"`
}

// ActiveOrderLeg represents a single leg from the database lineup JSONB.
type ActiveOrderLeg struct {
	MarketID string `json:"market_id"`
	Side     string `json:"side"`
}

// ActiveCancelRequest represents a cancel request recovered from the database.
type ActiveCancelRequest struct {
	OrderID   utils.UUID `json:"order_id"`
	CreatedAt string     `json:"created_at"`
}

// GetActiveExchangeState queries the database for all active exchange state (orders + cancel requests).
func (gw *Gateway) GetActiveExchangeState(ctx context.Context) (*ActiveExchangeState, error) {
	var resultJSON []byte
	err := gw.db.QueryRow(ctx, "SELECT get_active_exchange_state()").Scan(&resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get active exchange state: %w", err)
	}

	var state ActiveExchangeState
	if err := json.Unmarshal(resultJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse active exchange state: %w", err)
	}

	return &state, nil
}

// dbOrderTypeToProto converts a database exchange_order_type string to proto OrderType.
func dbOrderTypeToProto(dbType string) common.OrderType {
	switch gen.ExchangeOrder(dbType) {
	case gen.ExchangeOrderMarket:
		return common.OrderType_MARKET
	default:
		return common.OrderType_LIMIT
	}
}

// recoveryEvent is used to merge orders and cancel requests into a single sorted list by created_at.
type recoveryEvent struct {
	createdAt     string
	order         *ActiveExchangeOrder  // non-nil for order events
	cancelRequest *ActiveCancelRequest  // non-nil for cancel events
}

// recoverActiveState queries the database for active orders and cancel requests, populating gateway state.
// Called once during gateway startup, before connecting to the matching server.
func (gw *Gateway) recoverActiveState(ctx context.Context) error {
	state, err := gw.GetActiveExchangeState(ctx)
	if err != nil {
		return fmt.Errorf("failed to recover active state: %w", err)
	}

	if len(state.Orders) == 0 && len(state.CancelRequests) == 0 {
		log.Println("No active state to recover")
		return nil
	}

	log.Printf("Recovering active state from database: %d orders, %d cancel requests\n",
		len(state.Orders), len(state.CancelRequests))

	// Build a merged list of events sorted by created_at for LocalEventSequence assignment
	events := make([]recoveryEvent, 0, len(state.Orders)+len(state.CancelRequests))
	for i := range state.Orders {
		events = append(events, recoveryEvent{createdAt: state.Orders[i].CreatedAt, order: &state.Orders[i]})
	}
	for i := range state.CancelRequests {
		events = append(events, recoveryEvent{createdAt: state.CancelRequests[i].CreatedAt, cancelRequest: &state.CancelRequests[i]})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].createdAt < events[j].createdAt
	})

	// Process events in created_at order, assigning LocalEventSequence
	for _, event := range events {
		seq := gw.getNextLocalEventSequence()

		if event.order != nil {
			gw.recoverOrder(event.order, seq)
		} else if event.cancelRequest != nil {
			gw.recoverCancelRequest(event.cancelRequest, seq)
		}
	}

	log.Printf("Recovery complete: %d orders, %d cancel requests loaded\n",
		len(state.Orders), len(state.CancelRequests))
	return nil
}

// recoverOrder adds a single recovered order to the pool tracker and pending orders.
func (gw *Gateway) recoverOrder(order *ActiveExchangeOrder, seq uint64) {
	legs := make([]TrackedLeg, len(order.Legs))
	for i, leg := range order.Legs {
		marketID, err := utils.ParseUUID(leg.MarketID)
		if err != nil {
			log.Printf("WARNING: Failed to parse market ID %s for order %s: %v\n", leg.MarketID, order.OrderID.String(), err)
			continue
		}
		legs[i] = TrackedLeg{
			LegSecurityID: marketID,
			IsOver:        leg.Side == "over",
		}
	}

	orderType := dbOrderTypeToProto(order.OrderType)

	// Add to pool tracker
	gw.poolTracker.AddOrder(&TrackedOrder{
		DBRecordID:         order.OrderID,
		SlateID:            order.SlateID,
		Legs:               legs,
		Portion:            uint64(order.Portion),
		RemainingQty:       uint64(order.RemainingQuantity),
		OrderType:          orderType,
		UserID:             order.UserID,
		LineupIndex:        uint64(order.LineupIndex),
		LocalEventSequence: seq,
	})

	// Add to pending orders for submission to matching engine
	gw.pendingOrders[order.OrderID] = &PendingOrder{
		DBRecordID:         order.OrderID,
		UserID:             order.UserID,
		Quantity:           order.RemainingQuantity,
		Legs:               legs,
		Portion:            uint64(order.Portion),
		SlateID:            order.SlateID,
		IsRecovery:         true,
		OrderType:          orderType,
		LineupIndex:        uint64(order.LineupIndex),
		LocalEventSequence: seq,
	}

	log.Printf("Recovered order: id=%s, slate=%s, lineup=%d, remaining=%d\n",
		order.OrderID.String(), order.SlateID.String(), order.LineupIndex, order.RemainingQuantity)
}

// recoverCancelRequest adds a single recovered cancel request to pending cancels.
func (gw *Gateway) recoverCancelRequest(cancel *ActiveCancelRequest, seq uint64) {
	gw.pendingCancels[cancel.OrderID] = &PendingCancel{
		DBRecordID:         cancel.OrderID,
		IsRecovery:         true,
		LocalEventSequence: seq,
	}

	log.Printf("Recovered cancel request: orderId=%s\n", cancel.OrderID.String())
}

// resubmitOrdersToMatchingEngine sends DefinePool + NewOrder for all orders the gateway
// knows about. Called after connecting (or reconnecting) to the matching server.
func (gw *Gateway) resubmitOrdersToMatchingEngine() {
	// Clear engine-specific state (matching engine has reset)
	gw.confirmedOrdersMu.Lock()
	gw.confirmedOrders = make(map[uint64]*ConfirmedOrder)
	gw.confirmedOrdersMu.Unlock()

	gw.dbToEngineOrderIDMu.Lock()
	gw.dbToEngineOrderID = make(map[utils.UUID]uint64)
	gw.dbToEngineOrderIDMu.Unlock()

	gw.recoveryCancelsMu.Lock()
	gw.recoveryCancels = make(map[uint64]bool)
	gw.recoveryCancelsMu.Unlock()

	// Convert pool tracker orders into recovery pending orders
	trackerOrders := gw.poolTracker.GetAllOrders()

	gw.pendingOrdersMu.Lock()
	for _, order := range trackerOrders {
		// Only add if not already pending (avoid duplicates with existing pending orders)
		if _, exists := gw.pendingOrders[order.DBRecordID]; !exists {
			gw.pendingOrders[order.DBRecordID] = &PendingOrder{
				DBRecordID:         order.DBRecordID,
				UserID:             order.UserID,
				Quantity:           int64(order.RemainingQty),
				Legs:               order.Legs,
				Portion:            order.Portion,
				SlateID:            order.SlateID,
				IsRecovery:         true,
				OrderType:          order.OrderType,
				LineupIndex:        order.LineupIndex,
				LocalEventSequence: order.LocalEventSequence,
			}
		}
	}

	// Collect pending orders into two groups: recovery first, then non-recovery
	var recoveryOrders, normalOrders []*PendingOrder
	for _, p := range gw.pendingOrders {
		if p.IsRecovery {
			recoveryOrders = append(recoveryOrders, p)
		} else {
			normalOrders = append(normalOrders, p)
		}
	}
	gw.pendingOrdersMu.Unlock()

	// Sort each group by LocalEventSequence for deterministic FIFO ordering
	sort.Slice(recoveryOrders, func(i, j int) bool { return recoveryOrders[i].LocalEventSequence < recoveryOrders[j].LocalEventSequence })
	sort.Slice(normalOrders, func(i, j int) bool { return normalOrders[i].LocalEventSequence < normalOrders[j].LocalEventSequence })

	// Recovery orders first (DB created_at order), then normal pending orders (FIFO)
	pendingList := make([]*PendingOrder, 0, len(recoveryOrders)+len(normalOrders))
	pendingList = append(pendingList, recoveryOrders...)
	pendingList = append(pendingList, normalOrders...)

	if len(pendingList) == 0 {
		log.Println("No orders to resubmit to matching engine")
		return
	}

	log.Printf("Resubmitting %d orders to matching engine (%d recovery, %d pending)\n",
		len(pendingList), len(recoveryOrders), len(normalOrders))

	// Send DefinePool + NewOrder for each order
	for _, pending := range pendingList {
		gw.ensurePoolDefinedForResubmission(pending)

		selfMatchID := gw.GetSelfMatchID(context.Background(), pending.UserID)

		matchingOrderBody := &pb.NewOrder_Body{
			ClientOrderId: uuidToProto(pending.DBRecordID),
			SlateId:       uuidToProto(pending.SlateID),
			LineupIndex:   pending.LineupIndex,
			OrderType:     pending.OrderType,
			Portion:       pending.Portion,
			Quantity:      uint64(pending.Quantity),
		}
		if selfMatchID != nil {
			matchingOrderBody.SelfMatchId = uuidToProto(*selfMatchID)
		}

		upstreamMsg := &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: gw.getNextUpstreamSequenceNumber(),
			},
			Msg: &pb.GatewayMessage_NewOrder{
				NewOrder: &pb.NewOrder{Body: matchingOrderBody},
			},
		}

		gw.sendChan <- upstreamMsg
		log.Printf("Resubmitted order: dbRecordId=%s, slate=%s, lineup=%d, qty=%d\n",
			pending.DBRecordID.String(), pending.SlateID.String(), pending.LineupIndex, pending.Quantity)
	}
}

// ensurePoolDefinedForResubmission sends a DefinePool message for a pending order's slate
// if not yet defined on the current connection.
func (gw *Gateway) ensurePoolDefinedForResubmission(pending *PendingOrder) {
	slateIDStr := pending.SlateID.String()

	gw.definedPoolsMu.RLock()
	defined := gw.definedPools[slateIDStr]
	gw.definedPoolsMu.RUnlock()

	if defined {
		return
	}

	gw.definedPoolsMu.Lock()
	if gw.definedPools[slateIDStr] {
		gw.definedPoolsMu.Unlock()
		return
	}
	gw.definedPools[slateIDStr] = true
	gw.definedPoolsMu.Unlock()

	numLineups := uint64(1) << uint(len(pending.Legs))

	msg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextUpstreamSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_DefinePool{
			DefinePool: &pb.DefinePool{
				Body: &pb.DefinePool_Body{
					SlateId:    uuidToProto(pending.SlateID),
					TotalUnits: uint64(TotalUnits),
					NumLineups: numLineups,
				},
			},
		},
	}

	gw.sendChan <- msg
	log.Printf("Sent DefinePool for resubmission: slateId=%s (totalUnits=%d, numLineups=%d)\n",
		slateIDStr, TotalUnits, numLineups)
}

// handleRecoveryOrderAck processes a successful ack for a recovery/resubmission order.
func (gw *Gateway) handleRecoveryOrderAck(ctx context.Context, dbRecordID utils.UUID, engineOrderID uint64, pendingOrder *PendingOrder) {
	log.Printf("Recovery order acknowledged: dbRecordId=%s, engineOrderId=%d\n", dbRecordID.String(), engineOrderID)

	// Track confirmed order
	gw.confirmedOrdersMu.Lock()
	gw.confirmedOrders[engineOrderID] = &ConfirmedOrder{
		DBRecordID:           dbRecordID,
		UserID:               pendingOrder.UserID,
		BackendClientOrderID: pendingOrder.BackendClientOrderID,
	}
	gw.confirmedOrdersMu.Unlock()

	// Track reverse mapping
	gw.dbToEngineOrderIDMu.Lock()
	gw.dbToEngineOrderID[dbRecordID] = engineOrderID
	gw.dbToEngineOrderIDMu.Unlock()

	// Update DB status
	if err := gw.UpdateOrderStatus(ctx, dbRecordID, gen.ExchangeOrderStatusRestingOnExchange); err != nil {
		log.Printf("ERROR: Failed to update recovery order status: %v\n", err)
	}

	// Check for a pending cancel request for this order
	gw.pendingCancelsMu.Lock()
	pendingCancel, hasPendingCancel := gw.pendingCancels[dbRecordID]
	if hasPendingCancel {
		delete(gw.pendingCancels, dbRecordID)
	}
	gw.pendingCancelsMu.Unlock()

	if hasPendingCancel {
		gw.sendRecoveryCancelOrder(dbRecordID, engineOrderID, pendingCancel)
	}

	// Pool tracker already has this order — no need to add again
	// No client forwarding for recovery orders
}

// handleRecoveryOrderRejection processes a failed ack for a recovery/resubmission order.
func (gw *Gateway) handleRecoveryOrderRejection(ctx context.Context, dbRecordID utils.UUID, errorDesc string) {
	log.Printf("Recovery order rejected: dbRecordId=%s, error=%s\n", dbRecordID.String(), errorDesc)

	// Cancel in DB
	if err := gw.CancelExchangeOrderDueToExchange(ctx, dbRecordID); err != nil {
		log.Printf("ERROR: Failed to cancel rejected recovery order %s: %v\n", dbRecordID.String(), err)
	}

	// Remove from pool tracker (was added during recovery)
	slateID := gw.poolTracker.RemoveOrderAndGetSlate(dbRecordID)

	// Remove any pending cancel for this order (no longer relevant)
	gw.pendingCancelsMu.Lock()
	delete(gw.pendingCancels, dbRecordID)
	gw.pendingCancelsMu.Unlock()

	// Broadcast updated snapshot since pool state changed
	if slateID != (utils.UUID{}) {
		gw.broadcastPoolSnapshot(slateID)
	}

	// No client forwarding for recovery orders
}

// sendRecoveryCancelOrder sends a CancelOrder to the matching engine for a recovery cancel request.
func (gw *Gateway) sendRecoveryCancelOrder(dbRecordID utils.UUID, engineOrderID uint64, _ *PendingCancel) {
	log.Printf("Sending recovery cancel: dbRecordId=%s, engineOrderId=%d\n", dbRecordID.String(), engineOrderID)

	// Mark this engine order ID as a recovery cancel so the ack handler knows
	gw.recoveryCancelsMu.Lock()
	gw.recoveryCancels[engineOrderID] = true
	gw.recoveryCancelsMu.Unlock()

	upstreamMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextUpstreamSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_CancelOrder{
			CancelOrder: &pb.CancelOrder{
				Body: &pb.CancelOrder_Body{
					OrderId: engineOrderID,
				},
			},
		},
	}

	gw.sendChan <- upstreamMsg
}

// handleRecoveryCancelAck processes a successful cancel ack for a recovery cancel request.
func (gw *Gateway) handleRecoveryCancelAck(ctx context.Context, dbRecordID utils.UUID, engineOrderID uint64) {
	log.Printf("Recovery cancel acknowledged: dbRecordId=%s, engineOrderId=%d\n", dbRecordID.String(), engineOrderID)

	// Cancel the order in DB (user-initiated cancel)
	if err := gw.CancelExchangeOrderDueToUser(ctx, dbRecordID); err != nil {
		log.Printf("ERROR: Failed to cancel recovery order %s: %v\n", dbRecordID.String(), err)
	}

	// Remove from pool tracker and broadcast
	slateID := gw.poolTracker.RemoveOrderAndGetSlate(dbRecordID)

	gw.removeConfirmedOrder(engineOrderID, dbRecordID)

	if slateID != (utils.UUID{}) {
		gw.broadcastPoolSnapshot(slateID)
	}

	// No client forwarding for recovery cancels
}

// handleRecoveryCancelRejection processes a failed cancel ack for a recovery cancel request.
func (gw *Gateway) handleRecoveryCancelRejection(dbRecordID utils.UUID, engineOrderID uint64, errorDesc string) {
	log.Printf("Recovery cancel rejected: dbRecordId=%s, engineOrderId=%d, error=%s\n",
		dbRecordID.String(), engineOrderID, errorDesc)

	// Order remains active on the exchange — no further action needed
	// No client forwarding for recovery cancels
}
