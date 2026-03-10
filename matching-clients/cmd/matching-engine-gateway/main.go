package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	common "matching-clients/src/gen"
	pb "matching-clients/src/gen/matching"
	"matching-clients/src/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	gen "github.com/openbook/shared/models/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds configuration for the gateway
type Config struct {
	// Database Configuration
	PGHost     string
	PGPort     string
	PGDatabase string
	PGUser     string
	PGPassword string
	PGKeyPath  string

	// Matching Server Configuration (upstream Rust server)
	MatchingServerHost string
	MatchingServerPort string

	// Gateway gRPC Server Configuration (for incoming connections)
	GatewayPort string
}

// Gateway is the main matching engine gateway service
type Gateway struct {
	pb.UnimplementedMatchingServerServiceServer // Embed for forward compatibility

	config *Config
	db     *pgxpool.Pool
	conn   *grpc.ClientConn
	client pb.MatchingServerServiceClient

	// Unified bidirectional stream for communication with matching server
	tradeStream pb.MatchingServerService_CreateTradeStreamClient
	sendChan    chan *pb.GatewayMessage

	// Track pending orders by clientOrderId to map responses back
	pendingOrders   map[utils.UUID]*PendingOrder
	pendingOrdersMu sync.RWMutex

	// Track confirmed orders by matching engine orderId for fills/eliminations
	confirmedOrders   map[uint64]*ConfirmedOrder
	confirmedOrdersMu sync.RWMutex

	// Client stream tracking - maps clientOrderId to the client's response stream
	clientStreams   map[utils.UUID]pb.MatchingServerService_CreateTradeStreamServer
	clientStreamsMu sync.RWMutex

	// Sequence number for messages to upstream matching server
	sequenceNumber uint64
	sequenceMu     sync.Mutex

	// gRPC server for incoming connections
	grpcServer *grpc.Server

	// Upstream matching server connection state
	upstreamConnected bool
	upstreamMu        sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// PendingOrder tracks an order that has been submitted but not yet acknowledged
type PendingOrder struct {
	ClientOrderID utils.UUID
	UserID        utils.UUID
	Quantity      int64
	DBRecordID    utils.UUID // exchange_orders.id
}

// ConfirmedOrder tracks a confirmed order on the matching engine for fills/cancels/eliminations
type ConfirmedOrder struct {
	DBRecordID utils.UUID // exchange_orders.id
	UserID     utils.UUID
}

// OrderRequest represents an incoming order request from the app backend
type OrderRequest struct {
	UserID     utils.UUID
	MarketType gen.MarketEntity
	TotalUnits int64  // total_units for the exchange slate
	Legs       []LegRequest
	OrderType  common.OrderType
	Portion    uint64
	Quantity   uint64
}

// LegRequest represents a leg in an order.
// LegSecurityID serves as both the matching engine leg security ID and the database market ID.
type LegRequest struct {
	LegSecurityID *common.UUID
	IsOver        bool
}

// LegSecurityIDAsUUID converts the proto LegSecurityID to a utils.UUID.
func (l *LegRequest) LegSecurityIDAsUUID() utils.UUID {
	return utils.UUIDFromUint64(l.LegSecurityID.GetUpper(), l.LegSecurityID.GetLower())
}

// protoOrderTypeToEnum converts a proto OrderType to the database exchange_order_type enum.
func protoOrderTypeToEnum(ot common.OrderType) gen.ExchangeOrder {
	switch ot {
	case common.OrderType_MARKET:
		return gen.ExchangeOrderMarket
	default:
		return gen.ExchangeOrderLimit
	}
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Try to load .env file (non-fatal if not found)
	_ = godotenv.Load()

	cfg := &Config{
		PGHost:             os.Getenv("PG_HOST"),
		PGPort:             os.Getenv("PG_PORT"),
		PGDatabase:         os.Getenv("PG_DATABASE"),
		PGUser:             os.Getenv("PG_USER"),
		PGPassword:         os.Getenv("PG_PASSWORD"),
		PGKeyPath:          os.Getenv("PG_KEY_PATH"),
		MatchingServerHost: getEnvOrDefault("SERVER_HOST", "localhost"),
		MatchingServerPort: getEnvOrDefault("SERVER_PORT", "50051"),
		GatewayPort:        getEnvOrDefault("GATEWAY_PORT", "50052"),
	}

	// Validate required fields
	if cfg.PGHost == "" {
		return nil, errors.New("missing required environment variable: PG_HOST")
	}
	if cfg.PGPort == "" {
		return nil, errors.New("missing required environment variable: PG_PORT")
	}
	if cfg.PGDatabase == "" {
		return nil, errors.New("missing required environment variable: PG_DATABASE")
	}
	if cfg.PGUser == "" {
		return nil, errors.New("missing required environment variable: PG_USER")
	}
	if cfg.PGPassword == "" {
		return nil, errors.New("missing required environment variable: PG_PASSWORD")
	}
	if cfg.PGKeyPath == "" {
		return nil, errors.New("missing required environment variable: PG_KEY_PATH")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// NewGateway creates a new Gateway instance
func NewGateway(ctx context.Context, config *Config) (*Gateway, error) {
	ctx, cancel := context.WithCancel(ctx)

	gw := &Gateway{
		config:          config,
		pendingOrders:   make(map[utils.UUID]*PendingOrder),
		confirmedOrders: make(map[uint64]*ConfirmedOrder),
		clientStreams:   make(map[utils.UUID]pb.MatchingServerService_CreateTradeStreamServer),
		sendChan:        make(chan *pb.GatewayMessage, 100),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Connect to database (mandatory - fail fast)
	if err := gw.connectDB(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Connect to matching server (optional - gateway can start without it)
	if err := gw.connectMatchingServer(); err != nil {
		log.Printf("WARNING: Could not connect to matching server: %v\n", err)
		log.Println("Gateway will start without matching server connection and attempt to reconnect")
	}

	return gw, nil
}

// connectMatchingServer establishes a gRPC connection to the matching server
func (gw *Gateway) connectMatchingServer() error {
	addr := fmt.Sprintf("%s:%s", gw.config.MatchingServerHost, gw.config.MatchingServerPort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to matching server at %s: %w", addr, err)
	}

	gw.conn = conn
	gw.client = pb.NewMatchingServerServiceClient(conn)
	log.Printf("Connected to matching server at %s\n", addr)
	return nil
}

// Start begins the gateway operations
func (gw *Gateway) Start() error {
	// Try to initialize trade stream if we have a connection
	if gw.conn != nil {
		if err := gw.initializeTradeStream(); err != nil {
			log.Printf("WARNING: Could not initialize trade stream: %v\n", err)
		} else {
			gw.upstreamMu.Lock()
			gw.upstreamConnected = true
			gw.upstreamMu.Unlock()
			log.Println("Connected to matching server")
		}
	}

	// Start the gRPC server for incoming connections
	if err := gw.startGRPCServer(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start reconnection loop in background
	go gw.reconnectionLoop()

	log.Println("Gateway started successfully")
	return nil
}

// startGRPCServer starts the gRPC server for incoming client connections
func (gw *Gateway) startGRPCServer() error {
	addr := fmt.Sprintf(":%s", gw.config.GatewayPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	gw.grpcServer = grpc.NewServer()
	pb.RegisterMatchingServerServiceServer(gw.grpcServer, gw)

	// Start server in a goroutine
	go func() {
		log.Printf("Gateway gRPC server listening on %s\n", addr)
		if err := gw.grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v\n", err)
		}
	}()

	return nil
}

// initializeTradeStream sets up the unified bidirectional stream with the matching server
func (gw *Gateway) initializeTradeStream() error {
	stream, err := gw.client.CreateTradeStream(gw.ctx)
	if err != nil {
		return fmt.Errorf("failed to create trade stream: %w", err)
	}

	gw.tradeStream = stream

	// Goroutine to send messages from the channel
	go func() {
		for {
			select {
			case <-gw.ctx.Done():
				return
			case msg, ok := <-gw.sendChan:
				if !ok {
					return // channel closed
				}
				if err := stream.Send(msg); err != nil {
					log.Printf("Failed to send message to matching server: %v\n", err)
					gw.markDisconnected()
					return
				}
			}
		}
	}()

	// Goroutine to receive and process responses
	go gw.handleTradeStreamResponses()

	return nil
}

// getNextSequenceNumber returns the next sequence number for messages
func (gw *Gateway) getNextSequenceNumber() uint64 {
	gw.sequenceMu.Lock()
	defer gw.sequenceMu.Unlock()
	gw.sequenceNumber++
	return gw.sequenceNumber
}

// SubmitOrder validates and submits an order to the matching server
func (gw *Gateway) SubmitOrder(ctx context.Context, req *OrderRequest) error {
	if !gw.isUpstreamConnected() {
		return errors.New("matching server is not available")
	}

	// Step 1: Ensure slate and lineups exist, then find the matching lineup
	marketIDs := make([]utils.UUID, len(req.Legs))
	for i := range req.Legs {
		marketIDs[i] = req.Legs[i].LegSecurityIDAsUUID()
	}
	slateAndLineups, err := gw.EnsureSlateAndLineups(ctx, req.MarketType, marketIDs, req.TotalUnits)
	if err != nil {
		return fmt.Errorf("failed to ensure slate and lineups: %w", err)
	}

	lineupID, err := FindLineupID(slateAndLineups, req.Legs)
	if err != nil {
		return fmt.Errorf("failed to find lineup: %w", err)
	}

	// Step 2: Get next client order ID (using sequence number)
	seq := gw.getNextSequenceNumber()
	clientOrderID := utils.UUIDFromUint64(0, seq)
	clientOrderProto := &common.UUID{Upper: 0, Lower: seq}

	// Step 3: Create exchange order in DB (atomically deducts balance)
	dbRecordID, err := gw.CreateExchangeOrder(ctx, lineupID, req.UserID, protoOrderTypeToEnum(req.OrderType), req.Portion, req.Quantity, seq, gen.ExchangeOrderStatusReceivedByBackend)
	if err != nil {
		return fmt.Errorf("failed to create exchange order: %w", err)
	}

	// Step 4: Track the pending order
	gw.pendingOrdersMu.Lock()
	gw.pendingOrders[clientOrderID] = &PendingOrder{
		ClientOrderID: clientOrderID,
		UserID:        req.UserID,
		Quantity:      int64(req.Quantity),
		DBRecordID:    dbRecordID,
	}
	gw.pendingOrdersMu.Unlock()

	// Step 5: Get self match ID for user
	selfMatchID := gw.GetSelfMatchID(ctx, req.UserID)

	// Step 6: Build and send NewOrder via unified trade stream
	legs := make([]*pb.NewOrder_Body_Leg, len(req.Legs))
	for i, leg := range req.Legs {
		legs[i] = &pb.NewOrder_Body_Leg{
			LegSecurityId: leg.LegSecurityID,
			IsOver:        leg.IsOver,
		}
	}

	newOrderBody := &pb.NewOrder_Body{
		ClientOrderId: clientOrderProto,
		Legs:          legs,
		OrderType:     req.OrderType,
		Portion:       req.Portion,
		Quantity:      req.Quantity,
	}
	if selfMatchID != nil {
		newOrderBody.SelfMatchId = &common.UUID{
			Upper: selfMatchID.Upper(),
			Lower: selfMatchID.Lower(),
		}
	}

	newOrder := &pb.NewOrder{
		Body: newOrderBody,
	}

	gatewayMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_NewOrder{
			NewOrder: newOrder,
		},
	}

	// Send via channel (non-blocking with timeout)
	select {
	case gw.sendChan <- gatewayMsg:
		log.Printf("Submitted order clientOrderId=%s for user %s to matching server\n", clientOrderID.String(), req.UserID.String())
		if err := gw.UpdateOrderStatus(ctx, dbRecordID, gen.ExchangeOrderStatusSubmittedToExchange); err != nil {
			log.Printf("ERROR: Failed to update order status to submitted_to_exchange: %v\n", err)
		}
	case <-time.After(5 * time.Second):
		// Clean up on failure
		gw.pendingOrdersMu.Lock()
		delete(gw.pendingOrders, clientOrderID)
		gw.pendingOrdersMu.Unlock()
		// Atomically cancel and refund
		if err := gw.CancelExchangeOrderDueToExchange(ctx, dbRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel order %s on timeout: %v\n", dbRecordID.String(), err)
		}
		return errors.New("timeout sending order to matching server")
	}

	return nil
}

// handleTradeStreamResponses handles all responses from the unified trade stream
func (gw *Gateway) handleTradeStreamResponses() {
	for {
		select {
		case <-gw.ctx.Done():
			return
		default:
			resp, err := gw.tradeStream.Recv()
			if err == io.EOF {
				log.Println("Trade stream closed by server")
				gw.markDisconnected()
				return
			}
			if err != nil {
				log.Printf("Error receiving trade stream response: %v\n", err)
				gw.markDisconnected()
				return
			}

			gw.processEngineMessage(resp)
		}
	}
}

// forwardToClient sends an engine message to the connected client
func (gw *Gateway) forwardToClient(clientOrderID utils.UUID, msg *pb.EngineMessage) {
	gw.clientStreamsMu.RLock()
	clientStream, exists := gw.clientStreams[clientOrderID]
	gw.clientStreamsMu.RUnlock()

	if !exists {
		// No client connected for this order (may be internal order)
		return
	}

	if err := clientStream.Send(msg); err != nil {
		log.Printf("Failed to forward response to client for clientOrderId=%s: %v\n", clientOrderID.String(), err)
	}
}

// processEngineMessage processes a single EngineMessage from the matching server
func (gw *Gateway) processEngineMessage(resp *pb.EngineMessage) {
	ctx := context.Background()

	switch event := resp.Event.(type) {
	case *pb.EngineMessage_NewOrderAcknowledgement:
		gw.handleNewOrderAcknowledgement(ctx, event.NewOrderAcknowledgement)

	case *pb.EngineMessage_CancelOrderAcknowledgement:
		gw.handleCancelOrderAcknowledgement(ctx, event.CancelOrderAcknowledgement)

	case *pb.EngineMessage_Elimination:
		gw.handleOrderElimination(ctx, event.Elimination)

	case *pb.EngineMessage_Match:
		gw.handleMatch(ctx, event.Match)
	}
}

// handleNewOrderAcknowledgement processes a new order acknowledgement
func (gw *Gateway) handleNewOrderAcknowledgement(ctx context.Context, ack *pb.NewOrderAcknowledgement) {
	if ack.Body == nil {
		log.Println("ERROR: Received NewOrderAcknowledgement with nil body")
		return
	}

	clientOrderID := utils.UUIDFromUint64(ack.Body.ClientOrderId.GetUpper(), ack.Body.ClientOrderId.GetLower())
	orderID := ack.Body.OrderId

	gw.pendingOrdersMu.RLock()
	pendingOrder, exists := gw.pendingOrders[clientOrderID]
	gw.pendingOrdersMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Received acknowledgement for unknown clientOrderId=%s\n", clientOrderID.String())
		return
	}

	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		// Order accepted - balance was already deducted at submission time
		log.Printf("Order acknowledged: clientOrderId=%s, orderId=%d\n", clientOrderID.String(), orderID)

		// Track the confirmed order for later fills/eliminations
		gw.confirmedOrdersMu.Lock()
		gw.confirmedOrders[orderID] = &ConfirmedOrder{
			DBRecordID: pendingOrder.DBRecordID,
			UserID:     pendingOrder.UserID,
		}
		gw.confirmedOrdersMu.Unlock()

		// Update order status to resting
		if err := gw.UpdateOrderStatus(ctx, pendingOrder.DBRecordID, gen.ExchangeOrderStatusRestingOnExchange); err != nil {
			log.Printf("ERROR: Failed to update order status: %v\n", err)
		}
	} else {
		// Order rejected by matching server
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("Order rejected: clientOrderId=%s, error=%s\n", clientOrderID.String(), errorDesc)
		// Atomically cancel and refund (balance was deducted at order creation time)
		if err := gw.CancelExchangeOrderDueToExchange(ctx, pendingOrder.DBRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel rejected order %s: %v\n", pendingOrder.DBRecordID.String(), err)
		}
	}

	// Forward response to client if connected
	var seqNum uint64
	if ack.ResponseBase != nil {
		seqNum = ack.ResponseBase.RequestSequenceNumber
	}
	gw.forwardToClient(clientOrderID, &pb.EngineMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: seqNum,
		},
		Event: &pb.EngineMessage_NewOrderAcknowledgement{
			NewOrderAcknowledgement: ack,
		},
	})

	// Remove from pending orders and client streams
	gw.pendingOrdersMu.Lock()
	delete(gw.pendingOrders, clientOrderID)
	gw.pendingOrdersMu.Unlock()

	gw.clientStreamsMu.Lock()
	delete(gw.clientStreams, clientOrderID)
	gw.clientStreamsMu.Unlock()
}

// handleCancelOrderAcknowledgement processes a cancel order acknowledgement
func (gw *Gateway) handleCancelOrderAcknowledgement(ctx context.Context, ack *pb.CancelOrderAcknowledgement) {
	if ack.Body == nil {
		log.Println("ERROR: Received CancelOrderAcknowledgement with nil body")
		return
	}

	orderID := ack.Body.OrderId

	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		log.Printf("Order cancel acknowledged: orderId=%d\n", orderID)

		// Find the confirmed order info
		gw.confirmedOrdersMu.RLock()
		confirmed, exists := gw.confirmedOrders[orderID]
		gw.confirmedOrdersMu.RUnlock()

		if exists {
			// Atomically update status to cancelled_by_user and refund remaining balance
			if err := gw.CancelExchangeOrderDueToUser(ctx, confirmed.DBRecordID); err != nil {
				log.Printf("ERROR: Failed to cancel order: %v\n", err)
			}

			// Remove from confirmed orders
			gw.confirmedOrdersMu.Lock()
			delete(gw.confirmedOrders, orderID)
			gw.confirmedOrdersMu.Unlock()
		} else {
			log.Printf("WARNING: Received cancel ack for unknown orderId=%d\n", orderID)
		}
	} else {
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("Order cancel rejected: orderId=%d, error=%s\n", orderID, errorDesc)
	}
}

// handleOrderElimination processes an order elimination (server-initiated cancel)
func (gw *Gateway) handleOrderElimination(ctx context.Context, elim *pb.OrderElimination) {
	if elim.Body == nil {
		log.Println("ERROR: Received OrderElimination with nil body")
		return
	}

	orderID := elim.Body.OrderId
	reason := elim.Body.EliminationDescription

	log.Printf("Order eliminated: orderId=%d, reason=%s\n", orderID, reason)

	// Find the confirmed order info
	gw.confirmedOrdersMu.RLock()
	confirmed, exists := gw.confirmedOrders[orderID]
	gw.confirmedOrdersMu.RUnlock()

	if exists {
		// Atomically update status to cancelled_by_exchange and refund remaining balance
		if err := gw.CancelExchangeOrderDueToExchange(ctx, confirmed.DBRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel order: %v\n", err)
		}

		// Remove from confirmed orders
		gw.confirmedOrdersMu.Lock()
		delete(gw.confirmedOrders, orderID)
		gw.confirmedOrdersMu.Unlock()
	} else {
		log.Printf("WARNING: Received elimination for unknown orderId=%d\n", orderID)
	}
}

// handleMatch processes a match/fill event
// TODO: STUB - This needs atomic stored procedure for balance transfers
func (gw *Gateway) handleMatch(ctx context.Context, match *pb.Match) {
	if match.Body == nil {
		log.Println("ERROR: Received Match with nil body")
		return
	}

	log.Printf("Match received: transactionId=%d, matchId=%d, matchedQuantity=%d\n",
		match.Body.TransactionId, match.Body.MatchId, match.Body.MatchedQuantity)

	// Process each fill event
	for _, fillEvent := range match.Body.FillEvents {
		orderID := fillEvent.OrderId
		isComplete := fillEvent.IsComplete

		log.Printf("  FillEvent: fillEventId=%d, orderId=%d, isAggressor=%v, matchedPortion=%d, isComplete=%v\n",
			fillEvent.FillEventId, orderID, fillEvent.IsAggressor, fillEvent.MatchedPortion, isComplete)

		// Find the confirmed order info
		gw.confirmedOrdersMu.RLock()
		confirmed, exists := gw.confirmedOrders[orderID]
		gw.confirmedOrdersMu.RUnlock()

		if exists {
			if isComplete {
				// Order is fully filled
				if err := gw.UpdateOrderStatus(ctx, confirmed.DBRecordID, gen.ExchangeOrderStatusFullyFilled); err != nil {
					log.Printf("ERROR: Failed to update order status: %v\n", err)
				}

				// Remove from confirmed orders
				gw.confirmedOrdersMu.Lock()
				delete(gw.confirmedOrders, orderID)
				gw.confirmedOrdersMu.Unlock()
			}

			// TODO: STUB - Call stored procedure for atomic match settlement
			// This should:
			// 1. Update both orders involved in the match
			// 2. Transfer balances between users atomically
			// 3. Record the match/fill details
			log.Printf("TODO: Call stored procedure for atomic match settlement for order %d\n", orderID)
		} else {
			log.Printf("WARNING: Received fill for unknown orderId=%d\n", orderID)
		}
	}
}

// CancelOrder sends a cancel request for an order
func (gw *Gateway) CancelOrder(orderID uint64) error {
	if !gw.isUpstreamConnected() {
		return errors.New("matching server is not available")
	}

	cancelOrder := &pb.CancelOrder{
		Body: &pb.CancelOrder_Body{
			OrderId: orderID,
		},
	}

	gatewayMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_CancelOrder{
			CancelOrder: cancelOrder,
		},
	}

	// Send via channel (non-blocking with timeout)
	select {
	case gw.sendChan <- gatewayMsg:
		log.Printf("Sent cancel request for orderId=%d\n", orderID)
	case <-time.After(5 * time.Second):
		return errors.New("timeout sending cancel request")
	}

	return nil
}

// markDisconnected sets the upstream connection state to disconnected
func (gw *Gateway) markDisconnected() {
	gw.upstreamMu.Lock()
	wasConnected := gw.upstreamConnected
	gw.upstreamConnected = false
	gw.upstreamMu.Unlock()
	if wasConnected {
		log.Println("Disconnected from matching server")
	}
}

// isUpstreamConnected returns whether the upstream matching server is connected
func (gw *Gateway) isUpstreamConnected() bool {
	gw.upstreamMu.RLock()
	defer gw.upstreamMu.RUnlock()
	return gw.upstreamConnected
}

// reconnectionLoop runs in the background and attempts to reconnect when disconnected
func (gw *Gateway) reconnectionLoop() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-gw.ctx.Done():
			return
		case <-time.After(backoff):
		}

		if gw.isUpstreamConnected() {
			backoff = 1 * time.Second
			continue
		}

		log.Println("Attempting to reconnect to matching server...")

		if err := gw.attemptReconnect(); err != nil {
			log.Printf("Reconnection failed: %v\n", err)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		log.Println("Successfully reconnected to matching server")
		backoff = 1 * time.Second
	}
}

// attemptReconnect tries to establish a new connection to the matching server
func (gw *Gateway) attemptReconnect() error {
	// Close existing connection if any
	if gw.conn != nil {
		gw.conn.Close()
		gw.conn = nil
		gw.client = nil
	}

	// Create new gRPC connection
	if err := gw.connectMatchingServer(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// Reset sequence number for new stream
	gw.sequenceMu.Lock()
	gw.sequenceNumber = 0
	gw.sequenceMu.Unlock()

	// Drain any stale messages from send channel
	for {
		select {
		case <-gw.sendChan:
		default:
			goto drained
		}
	}
drained:

	// Initialize trade stream
	if err := gw.initializeTradeStream(); err != nil {
		gw.conn.Close()
		gw.conn = nil
		gw.client = nil
		return fmt.Errorf("stream creation failed: %w", err)
	}

	gw.upstreamMu.Lock()
	gw.upstreamConnected = true
	gw.upstreamMu.Unlock()

	return nil
}

// Shutdown gracefully shuts down the gateway
func (gw *Gateway) Shutdown() {
	log.Println("Shutting down gateway...")

	// Cancel context to stop all goroutines
	gw.cancel()

	// Stop gRPC server gracefully
	if gw.grpcServer != nil {
		gw.grpcServer.GracefulStop()
	}

	// Close send channel
	close(gw.sendChan)

	// Close trade stream
	if gw.tradeStream != nil {
		gw.tradeStream.CloseSend()
	}

	// Close gRPC connection to upstream
	if gw.conn != nil {
		gw.conn.Close()
	}

	// Close database connection
	if gw.db != nil {
		gw.db.Close()
	}

	log.Println("Gateway shutdown complete")
}

// CreateTradeStream implements the MatchingServerServiceServer interface.
// This handles incoming client connections and proxies them through gateway logic.
func (gw *Gateway) CreateTradeStream(stream pb.MatchingServerService_CreateTradeStreamServer) error {
	log.Println("New client connected to gateway")

	// Process incoming messages from the client
	for {
		select {
		case <-gw.ctx.Done():
			return gw.ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Client stream closed")
				return nil
			}
			if err != nil {
				log.Printf("Error receiving from client: %v\n", err)
				return err
			}

			// Process the message through gateway logic
			if err := gw.handleClientMessage(stream, msg); err != nil {
				log.Printf("Error handling client message: %v\n", err)
				// Don't return error - continue processing other messages
			}
		}
	}
}

// handleClientMessage processes a message from a connected client
func (gw *Gateway) handleClientMessage(clientStream pb.MatchingServerService_CreateTradeStreamServer, msg *pb.GatewayMessage) error {
	ctx := context.Background()

	switch m := msg.Msg.(type) {
	case *pb.GatewayMessage_NewOrder:
		return gw.handleClientNewOrder(ctx, clientStream, m.NewOrder)
	case *pb.GatewayMessage_CancelOrder:
		return gw.handleClientCancelOrder(ctx, clientStream, m.CancelOrder)
	default:
		log.Printf("Unknown message type from client\n")
		return nil
	}
}

// handleClientNewOrder processes a new order from a client through gateway logic
func (gw *Gateway) handleClientNewOrder(ctx context.Context, clientStream pb.MatchingServerService_CreateTradeStreamServer, order *pb.NewOrder) error {
	if order.Body == nil {
		return errors.New("order body is nil")
	}

	clientOrderProto := order.Body.ClientOrderId
	clientOrderID := utils.UUIDFromUint64(clientOrderProto.GetUpper(), clientOrderProto.GetLower())

	if !gw.isUpstreamConnected() {
		return gw.sendOrderRejection(clientStream, clientOrderProto, "matching server is not available")
	}

	log.Printf("Processing new order from client: clientOrderId=%s\n", clientOrderID.String())

	// TODO: Extract user info from gRPC metadata or session
	dummyUserID := utils.UUIDFromUint64(0, 1) // Placeholder UUID

	// Step 1: Build legs and resolve slate/lineup
	legs := make([]LegRequest, len(order.Body.Legs))
	for i, leg := range order.Body.Legs {
		legs[i] = LegRequest{
			LegSecurityID: leg.LegSecurityId,
			IsOver:        leg.IsOver,
		}
	}

	// TODO: MarketType and TotalUnits should come from the order or be derived
	marketType := gen.MarketEntityNbaMarket // Placeholder
	totalUnits := int64(len(legs))                  // Placeholder

	marketIDs := make([]utils.UUID, len(legs))
	for i := range legs {
		marketIDs[i] = legs[i].LegSecurityIDAsUUID()
	}
	slateAndLineups, err := gw.EnsureSlateAndLineups(ctx, marketType, marketIDs, totalUnits)
	if err != nil {
		log.Printf("Failed to ensure slate and lineups: %v\n", err)
		return gw.sendOrderRejection(clientStream, clientOrderProto, fmt.Sprintf("failed to ensure slate: %v", err))
	}

	lineupID, err := FindLineupID(slateAndLineups, legs)
	if err != nil {
		log.Printf("Failed to find lineup: %v\n", err)
		return gw.sendOrderRejection(clientStream, clientOrderProto, fmt.Sprintf("failed to find lineup: %v", err))
	}

	// Step 2: Create exchange order in DB (atomically deducts balance)
	dbRecordID, err := gw.CreateExchangeOrder(ctx, lineupID, dummyUserID, protoOrderTypeToEnum(order.Body.OrderType), order.Body.Portion, order.Body.Quantity, clientOrderID.Lower(), gen.ExchangeOrderStatusReceivedByBackend)

	if err != nil {
		log.Printf("Failed to create exchange order: %v\n", err)
		return gw.sendOrderRejection(clientStream, clientOrderProto, fmt.Sprintf("failed to create order: %v", err))
	}

	// Step 3: Track the pending order and client stream
	gw.pendingOrdersMu.Lock()
	gw.pendingOrders[clientOrderID] = &PendingOrder{
		ClientOrderID: clientOrderID,
		UserID:        dummyUserID,
		Quantity:      int64(order.Body.Quantity),
		DBRecordID:    dbRecordID,
	}
	gw.pendingOrdersMu.Unlock()

	gw.clientStreamsMu.Lock()
	gw.clientStreams[clientOrderID] = clientStream
	gw.clientStreamsMu.Unlock()

	// Step 4: Forward order to upstream matching server with gateway's sequence number
	gatewayMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_NewOrder{
			NewOrder: order,
		},
	}

	select {
	case gw.sendChan <- gatewayMsg:
		log.Printf("Forwarded order clientOrderId=%s to matching server\n", clientOrderID.String())
		if err := gw.UpdateOrderStatus(ctx, dbRecordID, gen.ExchangeOrderStatusSubmittedToExchange); err != nil {
			log.Printf("ERROR: Failed to update order status to submitted_to_exchange: %v\n", err)
		}
	case <-time.After(5 * time.Second):
		// Clean up on failure
		gw.pendingOrdersMu.Lock()
		delete(gw.pendingOrders, clientOrderID)
		gw.pendingOrdersMu.Unlock()
		gw.clientStreamsMu.Lock()
		delete(gw.clientStreams, clientOrderID)
		gw.clientStreamsMu.Unlock()
		// Atomically cancel and refund
		if err := gw.CancelExchangeOrderDueToExchange(ctx, dbRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel order %s on timeout: %v\n", dbRecordID.String(), err)
		}
		return gw.sendOrderRejection(clientStream, clientOrderProto, "timeout forwarding to matching server")
	}

	return nil
}

// handleClientCancelOrder processes a cancel order from a client
func (gw *Gateway) handleClientCancelOrder(ctx context.Context, clientStream pb.MatchingServerService_CreateTradeStreamServer, cancel *pb.CancelOrder) error {
	if cancel.Body == nil {
		return errors.New("cancel body is nil")
	}

	orderID := cancel.Body.OrderId

	if !gw.isUpstreamConnected() {
		return errors.New("matching server is not available")
	}

	log.Printf("Processing cancel order from client: orderId=%d\n", orderID)

	// Record the cancel request in the database
	gw.confirmedOrdersMu.RLock()
	confirmed, exists := gw.confirmedOrders[orderID]
	gw.confirmedOrdersMu.RUnlock()

	if exists {
		if err := gw.CreateExchangeCancelRequest(ctx, confirmed.DBRecordID); err != nil {
			log.Printf("ERROR: Failed to create cancel request for order %s: %v\n", confirmed.DBRecordID.String(), err)
		}
	} else {
		log.Printf("WARNING: Cancel request for unknown orderId=%d\n", orderID)
	}

	// Forward cancel to upstream matching server
	gatewayMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_CancelOrder{
			CancelOrder: cancel,
		},
	}

	select {
	case gw.sendChan <- gatewayMsg:
		log.Printf("Forwarded cancel orderId=%d to matching server\n", orderID)
	case <-time.After(5 * time.Second):
		return errors.New("timeout forwarding cancel to matching server")
	}

	return nil
}

// sendOrderRejection sends a rejection acknowledgement back to the client
func (gw *Gateway) sendOrderRejection(clientStream pb.MatchingServerService_CreateTradeStreamServer, clientOrderID *common.UUID, reason string) error {
	resp := &pb.EngineMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: 0, // Gateway-generated response
		},
		Event: &pb.EngineMessage_NewOrderAcknowledgement{
			NewOrderAcknowledgement: &pb.NewOrderAcknowledgement{
				FallibleBase: &common.FallibleBase{
					Success:          false,
					ErrorDescription: reason,
				},
				Body: &pb.NewOrderAcknowledgement_Body{
					ClientOrderId: clientOrderID,
					OrderId:       0,
				},
			},
		},
	}

	return clientStream.Send(resp)
}

// CreateDummyOrderRequest creates a dummy order request for testing
func CreateDummyOrderRequest(userID utils.UUID, quantity uint64) *OrderRequest {
	return &OrderRequest{
		UserID:     userID,
		MarketType: gen.MarketEntityNbaMarket,
		TotalUnits: 1,
		Legs: []LegRequest{
			{LegSecurityID: &common.UUID{Upper: 0, Lower: 1001}, IsOver: true},
		},
		OrderType: common.OrderType_LIMIT,
		Portion:   5000, // 50.00% as basis points
		Quantity:  quantity,
	}
}

func main() {
	log.Println("Starting Matching Engine Gateway...")

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v\n", err)
	}

	// Create gateway
	ctx := context.Background()
	gateway, err := NewGateway(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v\n", err)
	}
	defer gateway.Shutdown()

	// Start gateway
	if err := gateway.Start(); err != nil {
		log.Fatalf("Failed to start gateway: %v\n", err)
	}

	// For testing: Submit a dummy order
	// Uncomment the following to test:
	/*
	   dummyOrder := CreateDummyOrderRequest("auth0|634af2805615e6a1bdb53b3a", 100)
	   if err := gateway.SubmitOrder(ctx, dummyOrder); err != nil {
	   log.Printf("Failed to submit dummy order: %v\n", err)
	   }
	*/

	// Keep running until interrupted
	// In production, this would be replaced with proper signal handling
	log.Println("Gateway running. Press Ctrl+C to stop.")
	select {}
}
