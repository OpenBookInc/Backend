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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OrderStatus represents the status of an order in the database
type OrderStatus string

const (
	OrderStatusSubmitted   OrderStatus = "submitted_to_matching_engine"
	OrderStatusResting     OrderStatus = "resting_on_matching_engine"
	OrderStatusCancelledEx OrderStatus = "cancelled_by_exchange"
	OrderStatusCancelled   OrderStatus = "cancelled_by_user"
	OrderStatusFilled      OrderStatus = "fully_filled"
)

// BetStatus represents the status of a bet in the database
type BetStatus string

const (
	BetStatusPending   BetStatus = "pending"
	BetStatusLost      BetStatus = "lost"
	BetStatusPaid      BetStatus = "paid"
	BetStatusCancelled BetStatus = "cancelled"
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
	pendingOrders   map[uint64]*PendingOrder
	pendingOrdersMu sync.RWMutex

	// Track confirmed orders by matching engine orderId for fills/eliminations
	// Maps orderId (from matching engine) -> DBRecordID
	confirmedOrders   map[uint64]int64
	confirmedOrdersMu sync.RWMutex

	// Client stream tracking - maps clientOrderId to the client's response stream
	clientStreams   map[uint64]pb.MatchingServerService_CreateTradeStreamServer
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
	ClientOrderID uint64
	UserID        string
	WagerAmount   int64
	DBRecordID    int64 // The confirmed_bets.id
}

// OrderRequest represents an incoming order request from the app backend
// For now, we use dummy values for most fields
type OrderRequest struct {
	UserID      string
	WagerAmount int64
	// Dummy fields for now - will be populated by app backend later
	ButtonID    int64
	Line        int64
	TotalPayout int64
	Commission  int64
	// Proto fields - dummy values for now
	Legs      []LegRequest
	OrderType common.OrderType
	Portion   uint64
	Quantity  uint64
}

// LegRequest represents a leg in an order
type LegRequest struct {
	LegSecurityID uint64
	IsOver        bool
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
		pendingOrders:   make(map[uint64]*PendingOrder),
		confirmedOrders: make(map[uint64]int64),
		clientStreams:   make(map[uint64]pb.MatchingServerService_CreateTradeStreamServer),
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

	// Step 1: Check balance
	hasBalance, err := gw.CheckBalance(ctx, req.UserID, req.WagerAmount)
	if err != nil {
		return fmt.Errorf("balance check failed: %w", err)
	}
	if !hasBalance {
		return errors.New("insufficient balance")
	}

	// Step 2: Validate leg security IDs
	if err := gw.ValidateLegSecurityIDs(ctx, req.Legs); err != nil {
		return fmt.Errorf("leg validation failed: %w", err)
	}

	// Step 3: Get next client order ID (using sequence number)
	clientOrderID := gw.getNextSequenceNumber()

	// Step 4: Insert into confirmed_bets with status 'submitted_to_matching_engine'
	dbRecordID, err := gw.InsertConfirmedBet(ctx, req, clientOrderID)
	if err != nil {
		return fmt.Errorf("failed to insert bet record: %w", err)
	}

	// Step 5: Track the pending order
	gw.pendingOrdersMu.Lock()
	gw.pendingOrders[clientOrderID] = &PendingOrder{
		ClientOrderID: clientOrderID,
		UserID:        req.UserID,
		WagerAmount:   req.WagerAmount,
		DBRecordID:    dbRecordID,
	}
	gw.pendingOrdersMu.Unlock()

	// Step 6: Get self match ID for user
	selfMatchID, err := gw.GetSelfMatchID(ctx, req.UserID)
	if err != nil {
		log.Printf("WARNING: Failed to get self match ID for user %s: %v\n", req.UserID, err)
		// Continue without self-match protection
	}

	// Step 7: Build and send NewOrder via unified trade stream
	legs := make([]*pb.NewOrder_Body_Leg, len(req.Legs))
	for i, leg := range req.Legs {
		legs[i] = &pb.NewOrder_Body_Leg{
			LegSecurityId: leg.LegSecurityID,
			IsOver:        leg.IsOver,
		}
	}

	newOrder := &pb.NewOrder{
		Body: &pb.NewOrder_Body{
			ClientOrderId: clientOrderID,
			Legs:          legs,
			OrderType:     req.OrderType,
			Portion:       req.Portion,
			Quantity:      req.Quantity,
			SelfMatchId:   selfMatchID,
		},
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
		log.Printf("Submitted order clientOrderId=%d for user %s to matching server\n", clientOrderID, req.UserID)
	case <-time.After(5 * time.Second):
		// Clean up on failure
		gw.pendingOrdersMu.Lock()
		delete(gw.pendingOrders, clientOrderID)
		gw.pendingOrdersMu.Unlock()
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
func (gw *Gateway) forwardToClient(clientOrderID uint64, msg *pb.EngineMessage) {
	gw.clientStreamsMu.RLock()
	clientStream, exists := gw.clientStreams[clientOrderID]
	gw.clientStreamsMu.RUnlock()

	if !exists {
		// No client connected for this order (may be internal order)
		return
	}

	if err := clientStream.Send(msg); err != nil {
		log.Printf("Failed to forward response to client for clientOrderId=%d: %v\n", clientOrderID, err)
	}
}

// forwardToClientByOrderID sends an engine message to the client using the matching engine's orderId
func (gw *Gateway) forwardToClientByOrderID(orderID uint64, msg *pb.EngineMessage) {
	// We need to find which client this order belongs to
	// For now, we broadcast to all connected clients since we don't track orderId->clientStream mapping
	// TODO: Add orderId->clientOrderId mapping for proper routing
	gw.clientStreamsMu.RLock()
	defer gw.clientStreamsMu.RUnlock()

	for _, clientStream := range gw.clientStreams {
		if err := clientStream.Send(msg); err != nil {
			log.Printf("Failed to forward response to client: %v\n", err)
		}
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

	clientOrderID := ack.Body.ClientOrderId
	orderID := ack.Body.OrderId

	gw.pendingOrdersMu.RLock()
	pendingOrder, exists := gw.pendingOrders[clientOrderID]
	gw.pendingOrdersMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Received acknowledgement for unknown clientOrderId=%d\n", clientOrderID)
		return
	}

	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		// Order accepted - deduct balance and update status
		log.Printf("Order acknowledged: clientOrderId=%d, orderId=%d\n", clientOrderID, orderID)

		// Track the confirmed order for later fills/eliminations
		gw.confirmedOrdersMu.Lock()
		gw.confirmedOrders[orderID] = pendingOrder.DBRecordID
		gw.confirmedOrdersMu.Unlock()

		// Deduct balance from user
		if err := gw.DeductBalance(ctx, pendingOrder.UserID, pendingOrder.WagerAmount); err != nil {
			log.Printf("ERROR: Failed to deduct balance for user %s: %v\n", pendingOrder.UserID, err)
			// TODO: Handle this error case - maybe cancel the order?
		}

		// Update order status to resting
		if err := gw.UpdateOrderStatus(ctx, pendingOrder.DBRecordID, OrderStatusResting); err != nil {
			log.Printf("ERROR: Failed to update order status: %v\n", err)
		}
	} else {
		// Order rejected by matching server
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("Order rejected: clientOrderId=%d, error=%s\n", clientOrderID, errorDesc)
		// Don't deduct balance, order was not accepted
		// TODO: Consider updating order status to indicate rejection
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

		// Find the DB record ID for this order
		gw.confirmedOrdersMu.RLock()
		dbRecordID, exists := gw.confirmedOrders[orderID]
		gw.confirmedOrdersMu.RUnlock()

		if exists {
			// Update order status to cancelled_by_user
			if err := gw.UpdateOrderStatus(ctx, dbRecordID, OrderStatusCancelled); err != nil {
				log.Printf("ERROR: Failed to update order status: %v\n", err)
			}

			// Remove from confirmed orders
			gw.confirmedOrdersMu.Lock()
			delete(gw.confirmedOrders, orderID)
			gw.confirmedOrdersMu.Unlock()

			// TODO: STUB - Call stored procedure to refund balance atomically
			log.Printf("TODO: Call stored procedure to refund balance for cancelled order %d\n", orderID)
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

	// Find the DB record ID for this order
	gw.confirmedOrdersMu.RLock()
	dbRecordID, exists := gw.confirmedOrders[orderID]
	gw.confirmedOrdersMu.RUnlock()

	if exists {
		// Update order status to cancelled_by_exchange
		if err := gw.UpdateOrderStatus(ctx, dbRecordID, OrderStatusCancelledEx); err != nil {
			log.Printf("ERROR: Failed to update order status: %v\n", err)
		}

		// Remove from confirmed orders
		gw.confirmedOrdersMu.Lock()
		delete(gw.confirmedOrders, orderID)
		gw.confirmedOrdersMu.Unlock()

		// TODO: STUB - Call stored procedure to refund balance atomically
		log.Printf("TODO: Call stored procedure to refund balance for eliminated order %d\n", orderID)
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

		// Find the DB record ID for this order
		gw.confirmedOrdersMu.RLock()
		dbRecordID, exists := gw.confirmedOrders[orderID]
		gw.confirmedOrdersMu.RUnlock()

		if exists {
			if isComplete {
				// Order is fully filled
				if err := gw.UpdateOrderStatus(ctx, dbRecordID, OrderStatusFilled); err != nil {
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

	clientOrderID := order.Body.ClientOrderId

	if !gw.isUpstreamConnected() {
		return gw.sendOrderRejection(clientStream, clientOrderID, "matching server is not available")
	}

	log.Printf("Processing new order from client: clientOrderId=%d\n", clientOrderID)

	// For now, use dummy values for user/wager - in production these would come from auth/session
	// TODO: Extract user info from gRPC metadata or session
	dummyUserID := "test-user"
	dummyWagerAmount := int64(order.Body.Quantity)

	// Step 1: Check balance (using stub)
	hasBalance, err := gw.CheckBalance(ctx, dummyUserID, dummyWagerAmount)
	if err != nil {
		log.Printf("Balance check failed: %v\n", err)
		// Send rejection back to client
		return gw.sendOrderRejection(clientStream, clientOrderID, fmt.Sprintf("balance check failed: %v", err))
	}
	if !hasBalance {
		log.Printf("Insufficient balance for clientOrderId=%d\n", clientOrderID)
		return gw.sendOrderRejection(clientStream, clientOrderID, "insufficient balance")
	}

	// Step 2: Validate leg security IDs (using stub)
	legs := make([]LegRequest, len(order.Body.Legs))
	for i, leg := range order.Body.Legs {
		legs[i] = LegRequest{
			LegSecurityID: leg.LegSecurityId,
			IsOver:        leg.IsOver,
		}
	}
	if err := gw.ValidateLegSecurityIDs(ctx, legs); err != nil {
		log.Printf("Leg validation failed: %v\n", err)
		return gw.sendOrderRejection(clientStream, clientOrderID, fmt.Sprintf("leg validation failed: %v", err))
	}

	// Step 3: Create OrderRequest for database insertion
	orderReq := &OrderRequest{
		UserID:      dummyUserID,
		WagerAmount: dummyWagerAmount,
		ButtonID:    0, // Dummy
		Line:        0, // Dummy
		TotalPayout: dummyWagerAmount * 2,
		Commission:  0,
		Legs:        legs,
		OrderType:   order.Body.OrderType,
		Portion:     order.Body.Portion,
		Quantity:    order.Body.Quantity,
	}

	// Step 4: Insert into confirmed_bets (using stub)
	dbRecordID, err := gw.InsertConfirmedBet(ctx, orderReq, clientOrderID)
	if err != nil {
		log.Printf("Failed to insert bet record: %v\n", err)
		return gw.sendOrderRejection(clientStream, clientOrderID, fmt.Sprintf("failed to insert bet: %v", err))
	}

	// Step 5: Track the pending order and client stream
	gw.pendingOrdersMu.Lock()
	gw.pendingOrders[clientOrderID] = &PendingOrder{
		ClientOrderID: clientOrderID,
		UserID:        dummyUserID,
		WagerAmount:   dummyWagerAmount,
		DBRecordID:    dbRecordID,
	}
	gw.pendingOrdersMu.Unlock()

	// Track client stream for sending responses back
	gw.clientStreamsMu.Lock()
	gw.clientStreams[clientOrderID] = clientStream
	gw.clientStreamsMu.Unlock()

	// Step 6: Forward order to upstream matching server with gateway's sequence number
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
		log.Printf("Forwarded order clientOrderId=%d to matching server\n", clientOrderID)
	case <-time.After(5 * time.Second):
		// Clean up on failure
		gw.pendingOrdersMu.Lock()
		delete(gw.pendingOrders, clientOrderID)
		gw.pendingOrdersMu.Unlock()
		gw.clientStreamsMu.Lock()
		delete(gw.clientStreams, clientOrderID)
		gw.clientStreamsMu.Unlock()
		return gw.sendOrderRejection(clientStream, clientOrderID, "timeout forwarding to matching server")
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
func (gw *Gateway) sendOrderRejection(clientStream pb.MatchingServerService_CreateTradeStreamServer, clientOrderID uint64, reason string) error {
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
func CreateDummyOrderRequest(userID string, wagerAmount int64) *OrderRequest {
	return &OrderRequest{
		UserID:      userID,
		WagerAmount: wagerAmount,
		// Dummy values - will be provided by app backend later
		ButtonID:    12345,
		Line:        -110,
		TotalPayout: wagerAmount * 2,   // Placeholder calculation
		Commission:  wagerAmount / 100, // 1% commission placeholder
		Legs: []LegRequest{
			{LegSecurityID: 1001, IsOver: true},
		},
		OrderType: common.OrderType_LIMIT,
		Portion:   5000, // 50.00% as basis points
		Quantity:  uint64(wagerAmount),
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
