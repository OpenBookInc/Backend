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
	gwpb "matching-clients/src/gen/gateway"
	pb "matching-clients/src/gen/matching"
	"matching-clients/src/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	gen "github.com/openbook/shared/models/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TotalUnits is the fixed total units value for all exchange slates.
const TotalUnits int64 = 1_000_000

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

// Gateway is the main matching engine gateway service.
// It exposes GatewayServerService (gateway proto) for incoming client connections
// and uses MatchingServerServiceClient (matching proto) for upstream communication.
type Gateway struct {
	gwpb.UnimplementedGatewayServerServiceServer

	config *Config
	db     *pgxpool.Pool
	conn   *grpc.ClientConn
	client pb.MatchingServerServiceClient

	// Unified bidirectional stream for communication with matching server (matching proto)
	tradeStream pb.MatchingServerService_CreateTradeStreamClient
	sendChan    chan *pb.GatewayMessage

	// Pending orders: keyed by dbRecordID (UUID), which is also used as the
	// matching engine's client_order_id when forwarding upstream.
	pendingOrders   map[utils.UUID]*PendingOrder
	pendingOrdersMu sync.RWMutex

	// Confirmed orders: keyed by matching engine's order_id (uint64)
	confirmedOrders   map[uint64]*ConfirmedOrder
	confirmedOrdersMu sync.RWMutex

	// Reverse mapping: dbRecordID → matching engine order_id (for cancel routing)
	dbToEngineOrderID   map[utils.UUID]uint64
	dbToEngineOrderIDMu sync.RWMutex

	// Client stream tracking: keyed by dbRecordID (gateway proto stream)
	clientStreams   map[utils.UUID]gwpb.GatewayServerService_CreateTradeStreamServer
	clientStreamsMu sync.RWMutex

	// Sequence number for messages to upstream matching server
	upstreamSeqNumber uint64
	upstreamSeqMu     sync.Mutex

	// Sequence number for messages to downstream clients
	gatewaySeqNumber uint64
	gatewaySeqMu     sync.Mutex

	// gRPC server for incoming connections
	grpcServer *grpc.Server

	// Pools that have been defined on the current upstream connection (keyed by slate ID string)
	definedPools   map[string]bool
	definedPoolsMu sync.RWMutex

	// Upstream matching server connection state
	upstreamConnected bool
	upstreamMu        sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// PendingOrder tracks an order that has been submitted but not yet acknowledged
type PendingOrder struct {
	BackendClientOrderID uint64     // uint64 client_order_id from the backend client
	UserID               utils.UUID
	DBRecordID           utils.UUID // exchange_orders.id, also used as matching engine client_order_id
	Quantity             int64
}

// ConfirmedOrder tracks a confirmed order on the matching engine for fills/cancels/eliminations
type ConfirmedOrder struct {
	DBRecordID           utils.UUID // exchange_orders.id
	UserID               utils.UUID
	BackendClientOrderID uint64 // for forwarding responses to client
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

// uuidToProto converts a utils.UUID to a proto UUID.
func uuidToProto(id utils.UUID) *common.UUID {
	return &common.UUID{Upper: id.Upper(), Lower: id.Lower()}
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
		config:            config,
		pendingOrders:     make(map[utils.UUID]*PendingOrder),
		confirmedOrders:   make(map[uint64]*ConfirmedOrder),
		dbToEngineOrderID: make(map[utils.UUID]uint64),
		clientStreams:     make(map[utils.UUID]gwpb.GatewayServerService_CreateTradeStreamServer),
		definedPools:      make(map[string]bool),
		sendChan:          make(chan *pb.GatewayMessage, 100),
		ctx:               ctx,
		cancel:            cancel,
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
	gwpb.RegisterGatewayServerServiceServer(gw.grpcServer, gw)

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

// getNextUpstreamSequenceNumber returns the next sequence number for upstream messages
func (gw *Gateway) getNextUpstreamSequenceNumber() uint64 {
	gw.upstreamSeqMu.Lock()
	defer gw.upstreamSeqMu.Unlock()
	seq := gw.upstreamSeqNumber
	gw.upstreamSeqNumber++
	return seq
}

// getNextGatewaySequenceNumber returns the next sequence number for client-facing messages
func (gw *Gateway) getNextGatewaySequenceNumber() uint64 {
	gw.gatewaySeqMu.Lock()
	defer gw.gatewaySeqMu.Unlock()
	seq := gw.gatewaySeqNumber
	gw.gatewaySeqNumber++
	return seq
}

// handleTradeStreamResponses handles all responses from the upstream matching server
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

	case *pb.EngineMessage_DefinePoolAcknowledgement:
		gw.handleDefinePoolAcknowledgement(event.DefinePoolAcknowledgement)
	}
}

// handleDefinePoolAcknowledgement processes a define pool acknowledgement from the matching engine
func (gw *Gateway) handleDefinePoolAcknowledgement(ack *pb.DefinePoolAcknowledgement) {
	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		slateID := ""
		if ack.Body != nil && ack.Body.SlateId != nil {
			slateID = utils.UUIDFromUint64(ack.Body.SlateId.GetUpper(), ack.Body.SlateId.GetLower()).String()
		}
		log.Printf("Pool definition confirmed: slateId=%s\n", slateID)
	} else {
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("ERROR: Pool definition rejected: %s\n", errorDesc)
	}
}

// ensurePoolDefined sends a DefinePool message upstream if the pool for the given
// slate has not yet been defined on this connection. Stream ordering guarantees
// the DefinePool arrives before any subsequent NewOrder.
func (gw *Gateway) ensurePoolDefined(slateAndLineups *SlateAndLineups) {
	slateIDStr := slateAndLineups.Slate.ID.String()

	gw.definedPoolsMu.RLock()
	defined := gw.definedPools[slateIDStr]
	gw.definedPoolsMu.RUnlock()

	if defined {
		return
	}

	gw.definedPoolsMu.Lock()
	// Double-check under write lock
	if gw.definedPools[slateIDStr] {
		gw.definedPoolsMu.Unlock()
		return
	}
	gw.definedPools[slateIDStr] = true
	gw.definedPoolsMu.Unlock()

	msg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextUpstreamSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_DefinePool{
			DefinePool: &pb.DefinePool{
				Body: &pb.DefinePool_Body{
					SlateId:    uuidToProto(slateAndLineups.Slate.ID),
					TotalUnits: uint64(slateAndLineups.Slate.TotalUnits),
					NumLineups: uint64(len(slateAndLineups.Lineups)),
				},
			},
		},
	}

	gw.sendChan <- msg
	log.Printf("Sent DefinePool for slateId=%s (totalUnits=%d, numLineups=%d)\n",
		slateIDStr, slateAndLineups.Slate.TotalUnits, len(slateAndLineups.Lineups))
}

// forwardToClient sends a gateway proto message to the connected client for the given order
func (gw *Gateway) forwardToClient(dbRecordID utils.UUID, msg *gwpb.GatewayMessage) {
	gw.clientStreamsMu.RLock()
	clientStream, exists := gw.clientStreams[dbRecordID]
	gw.clientStreamsMu.RUnlock()

	if !exists {
		return
	}

	if err := clientStream.Send(msg); err != nil {
		log.Printf("Failed to forward response to client for dbRecordId=%s: %v\n", dbRecordID.String(), err)
	}
}

// handleNewOrderAcknowledgement processes a new order acknowledgement from the matching engine
func (gw *Gateway) handleNewOrderAcknowledgement(ctx context.Context, ack *pb.NewOrderAcknowledgement) {
	if ack.Body == nil {
		log.Println("ERROR: Received NewOrderAcknowledgement with nil body")
		return
	}

	// Matching engine's client_order_id = our dbRecordID
	dbRecordID := utils.UUIDFromUint64(ack.Body.ClientOrderId.GetUpper(), ack.Body.ClientOrderId.GetLower())
	engineOrderID := ack.Body.OrderId

	gw.pendingOrdersMu.RLock()
	pendingOrder, exists := gw.pendingOrders[dbRecordID]
	gw.pendingOrdersMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Received ack for unknown dbRecordId=%s\n", dbRecordID.String())
		return
	}

	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		log.Printf("Order acknowledged: dbRecordId=%s, engineOrderId=%d\n", dbRecordID.String(), engineOrderID)

		// Track confirmed order by engine's order_id
		gw.confirmedOrdersMu.Lock()
		gw.confirmedOrders[engineOrderID] = &ConfirmedOrder{
			DBRecordID:           dbRecordID,
			UserID:               pendingOrder.UserID,
			BackendClientOrderID: pendingOrder.BackendClientOrderID,
		}
		gw.confirmedOrdersMu.Unlock()

		// Track reverse mapping for cancel routing
		gw.dbToEngineOrderIDMu.Lock()
		gw.dbToEngineOrderID[dbRecordID] = engineOrderID
		gw.dbToEngineOrderIDMu.Unlock()

		// Update order status to resting
		if err := gw.UpdateOrderStatus(ctx, dbRecordID, gen.ExchangeOrderStatusRestingOnExchange); err != nil {
			log.Printf("ERROR: Failed to update order status: %v\n", err)
		}

		// Forward success ack to client
		gw.forwardToClient(dbRecordID, &gwpb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: gw.getNextGatewaySequenceNumber(),
			},
			Event: &gwpb.GatewayMessage_NewOrderAcknowledgement{
				NewOrderAcknowledgement: &gwpb.NewOrderAcknowledgement{
					FallibleBase: &common.FallibleBase{Success: true},
					Body: &gwpb.NewOrderAcknowledgement_Body{
						ClientOrderId: pendingOrder.BackendClientOrderID,
						OrderId:       uuidToProto(dbRecordID),
					},
				},
			},
		})
	} else {
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("Order rejected: dbRecordId=%s, error=%s\n", dbRecordID.String(), errorDesc)

		// Atomically cancel and refund
		if err := gw.CancelExchangeOrderDueToExchange(ctx, dbRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel rejected order %s: %v\n", dbRecordID.String(), err)
		}

		// Forward rejection to client
		gw.forwardToClient(dbRecordID, &gwpb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: gw.getNextGatewaySequenceNumber(),
			},
			Event: &gwpb.GatewayMessage_NewOrderAcknowledgement{
				NewOrderAcknowledgement: &gwpb.NewOrderAcknowledgement{
					FallibleBase: &common.FallibleBase{
						Success:          false,
						ErrorDescription: errorDesc,
					},
					Body: &gwpb.NewOrderAcknowledgement_Body{
						ClientOrderId: pendingOrder.BackendClientOrderID,
						OrderId:       uuidToProto(dbRecordID),
					},
				},
			},
		})

		// Clean up client stream for rejected order
		gw.clientStreamsMu.Lock()
		delete(gw.clientStreams, dbRecordID)
		gw.clientStreamsMu.Unlock()
	}

	// Remove from pending orders
	gw.pendingOrdersMu.Lock()
	delete(gw.pendingOrders, dbRecordID)
	gw.pendingOrdersMu.Unlock()
}

// handleCancelOrderAcknowledgement processes a cancel order acknowledgement from the matching engine
func (gw *Gateway) handleCancelOrderAcknowledgement(ctx context.Context, ack *pb.CancelOrderAcknowledgement) {
	if ack.Body == nil {
		log.Println("ERROR: Received CancelOrderAcknowledgement with nil body")
		return
	}

	engineOrderID := ack.Body.OrderId

	gw.confirmedOrdersMu.RLock()
	confirmed, exists := gw.confirmedOrders[engineOrderID]
	gw.confirmedOrdersMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Received cancel ack for unknown engineOrderId=%d\n", engineOrderID)
		return
	}

	if ack.FallibleBase != nil && ack.FallibleBase.Success {
		log.Printf("Order cancel acknowledged: engineOrderId=%d, dbRecordId=%s\n", engineOrderID, confirmed.DBRecordID.String())

		if err := gw.CancelExchangeOrderDueToUser(ctx, confirmed.DBRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel order: %v\n", err)
		}

		// Forward success to client
		gw.forwardToClient(confirmed.DBRecordID, &gwpb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: gw.getNextGatewaySequenceNumber(),
			},
			Event: &gwpb.GatewayMessage_CancelOrderAcknowledgement{
				CancelOrderAcknowledgement: &gwpb.CancelOrderAcknowledgement{
					FallibleBase: &common.FallibleBase{Success: true},
					Body: &gwpb.CancelOrderAcknowledgement_Body{
						OrderId: uuidToProto(confirmed.DBRecordID),
					},
				},
			},
		})

		gw.removeConfirmedOrder(engineOrderID, confirmed.DBRecordID)
	} else {
		errorDesc := ""
		if ack.FallibleBase != nil {
			errorDesc = ack.FallibleBase.ErrorDescription
		}
		log.Printf("Order cancel rejected: engineOrderId=%d, error=%s\n", engineOrderID, errorDesc)

		// Forward rejection to client
		gw.forwardToClient(confirmed.DBRecordID, &gwpb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: gw.getNextGatewaySequenceNumber(),
			},
			Event: &gwpb.GatewayMessage_CancelOrderAcknowledgement{
				CancelOrderAcknowledgement: &gwpb.CancelOrderAcknowledgement{
					FallibleBase: &common.FallibleBase{
						Success:          false,
						ErrorDescription: errorDesc,
					},
					Body: &gwpb.CancelOrderAcknowledgement_Body{
						OrderId: uuidToProto(confirmed.DBRecordID),
					},
				},
			},
		})
	}
}

// handleOrderElimination processes an order elimination (server-initiated cancel) from the matching engine
func (gw *Gateway) handleOrderElimination(ctx context.Context, elim *pb.OrderElimination) {
	if elim.Body == nil {
		log.Println("ERROR: Received OrderElimination with nil body")
		return
	}

	engineOrderID := elim.Body.OrderId
	log.Printf("Order eliminated: engineOrderId=%d, reason=%s\n", engineOrderID, elim.Body.EliminationDescription)

	gw.confirmedOrdersMu.RLock()
	confirmed, exists := gw.confirmedOrders[engineOrderID]
	gw.confirmedOrdersMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Received elimination for unknown engineOrderId=%d\n", engineOrderID)
		return
	}

	if err := gw.CancelExchangeOrderDueToExchange(ctx, confirmed.DBRecordID); err != nil {
		log.Printf("ERROR: Failed to cancel order: %v\n", err)
	}

	// Forward elimination to client
	gw.forwardToClient(confirmed.DBRecordID, &gwpb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextGatewaySequenceNumber(),
		},
		Event: &gwpb.GatewayMessage_Elimination{
			Elimination: &gwpb.OrderElimination{
				Body: &gwpb.OrderElimination_Body{
					OrderId: uuidToProto(confirmed.DBRecordID),
				},
			},
		},
	})

	gw.removeConfirmedOrder(engineOrderID, confirmed.DBRecordID)
}

// handleMatch processes a match/fill event from the matching engine.
// It records the match in the database and forwards the result to connected clients.
func (gw *Gateway) handleMatch(ctx context.Context, match *pb.Match) {
	if match.Body == nil {
		log.Println("ERROR: Received Match with nil body")
		return
	}

	log.Printf("Match received: transactionId=%d, matchId=%d, matchedQuantity=%d\n",
		match.Body.TransactionId, match.Body.MatchId, match.Body.MatchedQuantity)

	// Resolve all fill events: engine order IDs → DB record IDs
	type resolvedFill struct {
		confirmed      *ConfirmedOrder
		engineOrderID  uint64
		isAggressor    bool
		matchedPortion uint64
		isComplete     bool
	}

	var fills []resolvedFill
	var aggressorDBID utils.UUID

	gw.confirmedOrdersMu.RLock()
	for _, fe := range match.Body.FillEvents {
		confirmed, exists := gw.confirmedOrders[fe.OrderId]
		if !exists {
			log.Printf("WARNING: Received fill for unknown engineOrderId=%d\n", fe.OrderId)
			continue
		}
		fills = append(fills, resolvedFill{
			confirmed:      confirmed,
			engineOrderID:  fe.OrderId,
			isAggressor:    fe.IsAggressor,
			matchedPortion: fe.MatchedPortion,
			isComplete:     fe.IsComplete,
		})
		if fe.IsAggressor {
			aggressorDBID = confirmed.DBRecordID
		}

		log.Printf("  FillEvent: engineOrderId=%d, dbRecordId=%s, isAggressor=%v, matchedPortion=%d, isComplete=%v\n",
			fe.OrderId, confirmed.DBRecordID.String(), fe.IsAggressor, fe.MatchedPortion, fe.IsComplete)
	}
	gw.confirmedOrdersMu.RUnlock()

	if len(fills) == 0 {
		return
	}

	// Build fills parameter for DB function
	dbFills := make([]MatchFillParam, len(fills))
	for i, f := range fills {
		dbFills[i] = MatchFillParam{
			OrderID:        f.confirmed.DBRecordID.String(),
			MatchedPortion: int64(f.matchedPortion),
		}
	}

	// Record match in database
	matchResult, err := gw.CreateExchangeMatch(ctx, aggressorDBID, int64(match.Body.MatchedQuantity), dbFills)
	if err != nil {
		log.Printf("ERROR: Failed to create exchange match: %v\n", err)
		return
	}

	// Build gateway proto fill events using DB-generated IDs
	gwFillEvents := make([]*gwpb.Match_Body_FillEvent, len(matchResult.Fills))
	for i, dbFill := range matchResult.Fills {
		// Find the original fill to get isAggressor/isComplete
		var isAggressor, isComplete bool
		for _, f := range fills {
			if f.confirmed.DBRecordID == dbFill.OrderID {
				isAggressor = f.isAggressor
				isComplete = f.isComplete
				break
			}
		}
		gwFillEvents[i] = &gwpb.Match_Body_FillEvent{
			FillEventId:    uuidToProto(dbFill.FillID),
			OrderId:        uuidToProto(dbFill.OrderID),
			IsAggressor:    isAggressor,
			MatchedPortion: uint64(dbFill.MatchedPortion),
			IsComplete:     isComplete,
		}
	}

	gwMatch := &gwpb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextGatewaySequenceNumber(),
		},
		Event: &gwpb.GatewayMessage_Match{
			Match: &gwpb.Match{
				Body: &gwpb.Match_Body{
					TransactionId:   match.Body.TransactionId,
					MatchId:         uuidToProto(matchResult.MatchID),
					MatchedQuantity: match.Body.MatchedQuantity,
					FillEvents:      gwFillEvents,
				},
			},
		},
	}

	// Forward match to each involved client
	notified := make(map[utils.UUID]bool)
	for _, f := range fills {
		dbID := f.confirmed.DBRecordID
		if !notified[dbID] {
			gw.forwardToClient(dbID, gwMatch)
			notified[dbID] = true
		}
	}

	// Clean up completed orders
	for _, f := range fills {
		if f.isComplete {
			gw.removeConfirmedOrder(f.engineOrderID, f.confirmed.DBRecordID)
		}
	}
}

// removeConfirmedOrder cleans up all tracking state for a completed/cancelled/eliminated order
func (gw *Gateway) removeConfirmedOrder(engineOrderID uint64, dbRecordID utils.UUID) {
	gw.confirmedOrdersMu.Lock()
	delete(gw.confirmedOrders, engineOrderID)
	gw.confirmedOrdersMu.Unlock()

	gw.dbToEngineOrderIDMu.Lock()
	delete(gw.dbToEngineOrderID, dbRecordID)
	gw.dbToEngineOrderIDMu.Unlock()

	gw.clientStreamsMu.Lock()
	delete(gw.clientStreams, dbRecordID)
	gw.clientStreamsMu.Unlock()
}

// CreateTradeStream implements the GatewayServerServiceServer interface.
// This handles incoming client connections using the gateway proto.
func (gw *Gateway) CreateTradeStream(stream gwpb.GatewayServerService_CreateTradeStreamServer) error {
	log.Println("New client connected to gateway")

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

			if err := gw.handleClientMessage(stream, msg); err != nil {
				log.Printf("Error handling client message: %v\n", err)
			}
		}
	}
}

// handleClientMessage processes a BackendMessage from a connected client
func (gw *Gateway) handleClientMessage(clientStream gwpb.GatewayServerService_CreateTradeStreamServer, msg *gwpb.BackendMessage) error {
	ctx := context.Background()

	switch m := msg.Msg.(type) {
	case *gwpb.BackendMessage_NewOrder:
		return gw.handleClientNewOrder(ctx, clientStream, m.NewOrder)
	case *gwpb.BackendMessage_CancelOrder:
		return gw.handleClientCancelOrder(ctx, clientStream, m.CancelOrder)
	default:
		log.Printf("Unknown message type from client\n")
		return nil
	}
}

// handleClientNewOrder processes a new order from a client, creates a DB record,
// and forwards to the upstream matching server using the matching proto.
func (gw *Gateway) handleClientNewOrder(ctx context.Context, clientStream gwpb.GatewayServerService_CreateTradeStreamServer, order *gwpb.NewOrder) error {
	if order.Body == nil {
		return errors.New("order body is nil")
	}

	backendClientOrderID := order.Body.ClientOrderId
	userID := utils.UUIDFromUint64(order.Body.UserId.GetUpper(), order.Body.UserId.GetLower())

	if !gw.isUpstreamConnected() {
		return gw.sendOrderRejection(clientStream, backendClientOrderID, "matching server is not available")
	}

	log.Printf("Processing new order from client: clientOrderId=%d, userId=%s\n", backendClientOrderID, userID.String())

	// Step 1: Build legs and resolve slate/lineup
	legs := make([]LegRequest, len(order.Body.Legs))
	for i, leg := range order.Body.Legs {
		legs[i] = LegRequest{
			LegSecurityID: leg.LegSecurityId,
			IsOver:        leg.IsOver,
		}
	}

	totalUnits := TotalUnits
	marketIDs := make([]utils.UUID, len(legs))
	for i := range legs {
		marketIDs[i] = legs[i].LegSecurityIDAsUUID()
	}
	slateAndLineups, err := gw.EnsureSlateAndLineups(ctx, marketIDs, totalUnits)
	if err != nil {
		log.Printf("Failed to ensure slate and lineups: %v\n", err)
		return gw.sendOrderRejection(clientStream, backendClientOrderID, fmt.Sprintf("failed to ensure slate: %v", err))
	}

	lineupID, lineupIndex, err := FindLineup(slateAndLineups, legs)
	if err != nil {
		log.Printf("Failed to find lineup: %v\n", err)
		return gw.sendOrderRejection(clientStream, backendClientOrderID, fmt.Sprintf("failed to find lineup: %v", err))
	}

	// Ensure the pool is defined on the upstream matching server before sending orders
	gw.ensurePoolDefined(slateAndLineups)

	// Step 2: Create exchange order in DB (atomically deducts balance)
	dbRecordID, err := gw.CreateExchangeOrder(ctx, lineupID, userID, protoOrderTypeToEnum(order.Body.OrderType), order.Body.Portion, order.Body.Quantity, backendClientOrderID, gen.ExchangeOrderStatusReceivedByBackend)
	if err != nil {
		log.Printf("Failed to create exchange order: %v\n", err)
		return gw.sendOrderRejection(clientStream, backendClientOrderID, fmt.Sprintf("failed to create order: %v", err))
	}

	// Step 3: Track the pending order and client stream
	gw.pendingOrdersMu.Lock()
	gw.pendingOrders[dbRecordID] = &PendingOrder{
		BackendClientOrderID: backendClientOrderID,
		UserID:               userID,
		DBRecordID:           dbRecordID,
		Quantity:             int64(order.Body.Quantity),
	}
	gw.pendingOrdersMu.Unlock()

	gw.clientStreamsMu.Lock()
	gw.clientStreams[dbRecordID] = clientStream
	gw.clientStreamsMu.Unlock()

	// Step 4: Get self match ID for user
	selfMatchID := gw.GetSelfMatchID(ctx, userID)

	// Step 5: Build matching proto NewOrder for upstream
	matchingOrderBody := &pb.NewOrder_Body{
		ClientOrderId: uuidToProto(dbRecordID),
		SlateId:       uuidToProto(slateAndLineups.Slate.ID),
		LineupIndex:   uint64(lineupIndex),
		OrderType:     order.Body.OrderType,
		Portion:       order.Body.Portion,
		Quantity:      order.Body.Quantity,
	}
	if selfMatchID != nil {
		matchingOrderBody.SelfMatchId = &common.UUID{
			Upper: selfMatchID.Upper(),
			Lower: selfMatchID.Lower(),
		}
	}

	upstreamMsg := &pb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: gw.getNextUpstreamSequenceNumber(),
		},
		Msg: &pb.GatewayMessage_NewOrder{
			NewOrder: &pb.NewOrder{Body: matchingOrderBody},
		},
	}

	// Step 6: Send to matching server
	select {
	case gw.sendChan <- upstreamMsg:
		log.Printf("Forwarded order dbRecordId=%s to matching server\n", dbRecordID.String())
		if err := gw.UpdateOrderStatus(ctx, dbRecordID, gen.ExchangeOrderStatusSubmittedToExchange); err != nil {
			log.Printf("ERROR: Failed to update order status to submitted_to_exchange: %v\n", err)
		}
	case <-time.After(5 * time.Second):
		// Clean up on failure
		gw.pendingOrdersMu.Lock()
		delete(gw.pendingOrders, dbRecordID)
		gw.pendingOrdersMu.Unlock()
		gw.clientStreamsMu.Lock()
		delete(gw.clientStreams, dbRecordID)
		gw.clientStreamsMu.Unlock()
		if err := gw.CancelExchangeOrderDueToExchange(ctx, dbRecordID); err != nil {
			log.Printf("ERROR: Failed to cancel order %s on timeout: %v\n", dbRecordID.String(), err)
		}
		return gw.sendOrderRejection(clientStream, backendClientOrderID, "timeout forwarding to matching server")
	}

	return nil
}

// handleClientCancelOrder processes a cancel order from a client.
// The client sends a UUID order_id (= dbRecordID), which is translated to
// the matching engine's uint64 order_id for upstream forwarding.
func (gw *Gateway) handleClientCancelOrder(ctx context.Context, clientStream gwpb.GatewayServerService_CreateTradeStreamServer, cancel *gwpb.CancelOrder) error {
	if cancel.Body == nil {
		return errors.New("cancel body is nil")
	}

	dbRecordID := utils.UUIDFromUint64(cancel.Body.OrderId.GetUpper(), cancel.Body.OrderId.GetLower())

	if !gw.isUpstreamConnected() {
		return errors.New("matching server is not available")
	}

	log.Printf("Processing cancel order from client: dbRecordId=%s\n", dbRecordID.String())

	// Look up the matching engine's order ID
	gw.dbToEngineOrderIDMu.RLock()
	engineOrderID, exists := gw.dbToEngineOrderID[dbRecordID]
	gw.dbToEngineOrderIDMu.RUnlock()

	if !exists {
		log.Printf("WARNING: Cancel request for unknown dbRecordId=%s\n", dbRecordID.String())
		return fmt.Errorf("order not found: %s", dbRecordID.String())
	}

	// Record the cancel request in the database
	if err := gw.CreateExchangeCancelRequest(ctx, dbRecordID); err != nil {
		log.Printf("ERROR: Failed to create cancel request for order %s: %v\n", dbRecordID.String(), err)
	}

	// Forward cancel to upstream matching server using matching proto
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

	select {
	case gw.sendChan <- upstreamMsg:
		log.Printf("Forwarded cancel dbRecordId=%s (engineOrderId=%d) to matching server\n", dbRecordID.String(), engineOrderID)
	case <-time.After(5 * time.Second):
		return errors.New("timeout forwarding cancel to matching server")
	}

	return nil
}

// sendOrderRejection sends a rejection acknowledgement back to the client using the gateway proto
func (gw *Gateway) sendOrderRejection(clientStream gwpb.GatewayServerService_CreateTradeStreamServer, backendClientOrderID uint64, reason string) error {
	resp := &gwpb.GatewayMessage{
		SequencedMessageBase: &common.SequencedMessageBase{
			SequenceNumber: 0,
		},
		Event: &gwpb.GatewayMessage_NewOrderAcknowledgement{
			NewOrderAcknowledgement: &gwpb.NewOrderAcknowledgement{
				FallibleBase: &common.FallibleBase{
					Success:          false,
					ErrorDescription: reason,
				},
				Body: &gwpb.NewOrderAcknowledgement_Body{
					ClientOrderId: backendClientOrderID,
				},
			},
		},
	}

	return clientStream.Send(resp)
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

	// Reset upstream sequence number for new stream
	gw.upstreamSeqMu.Lock()
	gw.upstreamSeqNumber = 0
	gw.upstreamSeqMu.Unlock()

	// Drain any stale messages from send channel
	for {
		select {
		case <-gw.sendChan:
		default:
			goto drained
		}
	}
drained:

	// Clear defined pools for the new connection
	gw.definedPoolsMu.Lock()
	gw.definedPools = make(map[string]bool)
	gw.definedPoolsMu.Unlock()

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

	// Keep running until interrupted
	log.Println("Gateway running. Press Ctrl+C to stop.")
	select {}
}
