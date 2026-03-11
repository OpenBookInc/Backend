package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	common "matching-clients/src/gen"
	pb "matching-clients/src/gen/matching"
	tc "matching-clients/src/tester_common"
	"github.com/openbook/shared/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:embed index.html
var indexHTML string

type WebClient struct {
	conn             *grpc.ClientConn
	mu               sync.RWMutex
	matchingClient   pb.MatchingServerServiceClient
	matchingStream   pb.MatchingServerService_CreateTradeStreamClient
	matchingSendChan chan *pb.GatewayMessage

	// Unified request/response storage
	requests  []string
	responses []string

	tradeStreamActive bool
	streamGeneration  uint64 // Incremented on each reconnect to invalidate old goroutines

	// Global sequence number shared across all message types
	globalSequenceNumber uint64

	// Global client order ID counter
	globalClientOrderId uint64

	// Pool state tracking
	poolTracker   *tc.PoolTracker
	pendingOrders map[uint64]*tc.PendingOrderInfo // map[clientOrderId]PendingOrderInfo

	// Connection info
	serverHost string
	serverPort string
}

func NewWebClient(cfg *tc.Config) *WebClient {
	wc := &WebClient{
		requests:            make([]string, 0),
		responses:           make([]string, 0),
		tradeStreamActive:   false,
		globalClientOrderId: 1000, // Start at 1001 (will increment on first use)
		poolTracker:         tc.NewPoolTracker(),
		pendingOrders:       make(map[uint64]*tc.PendingOrderInfo),
		serverHost:          cfg.ServerHost,
		serverPort:          cfg.ServerPort,
	}

	// Try to connect to the matching server, but don't fail if unreachable
	if err := wc.Connect(); err != nil {
		log.Printf("Warning: Could not connect to matching server: %v", err)
		log.Printf("Web server will start in disconnected state")
	}

	return wc
}

// processEngineMessage processes matching server responses and updates pool state
// NOTE: This method assumes wc.mu is already locked by the caller
func (wc *WebClient) processEngineMessage(resp *pb.EngineMessage) {
	switch event := resp.Event.(type) {
	case *pb.EngineMessage_NewOrderAcknowledgement:
		ack := event.NewOrderAcknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			clientOrderID := ack.Body.ClientOrderId.GetLower()
			orderIDStr := strconv.FormatUint(ack.Body.OrderId, 10)
			var sequenceNumber uint64
			if resp.SequencedMessageBase != nil {
				sequenceNumber = resp.SequencedMessageBase.SequenceNumber
			}

			if pendingOrder, exists := wc.pendingOrders[clientOrderID]; exists {
				if pendingOrder.SlateID != "" {
					wc.poolTracker.AddOrderBySlateID(
						orderIDStr,
						clientOrderID,
						pendingOrder.SlateID,
						pendingOrder.LineupIndex,
						pendingOrder.Portion,
						pendingOrder.Quantity,
						sequenceNumber,
					)
				} else {
					wc.poolTracker.AddOrder(
						orderIDStr,
						clientOrderID,
						pendingOrder.Legs,
						pendingOrder.Portion,
						pendingOrder.Quantity,
						sequenceNumber,
					)
				}
				delete(wc.pendingOrders, clientOrderID)
			}
		}

	case *pb.EngineMessage_CancelOrderAcknowledgement:
		ack := event.CancelOrderAcknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			wc.poolTracker.RemoveOrder(strconv.FormatUint(ack.Body.OrderId, 10))
		}

	case *pb.EngineMessage_Elimination:
		elim := event.Elimination
		if elim != nil && elim.Body != nil {
			wc.poolTracker.RemoveOrder(strconv.FormatUint(elim.Body.OrderId, 10))
		}

	case *pb.EngineMessage_Match:
		match := event.Match
		if match.Body != nil {
			for _, fillEvent := range match.Body.FillEvents {
				wc.poolTracker.UpdateFromFill(
					strconv.FormatUint(fillEvent.OrderId, 10),
					match.Body.MatchedQuantity,
					fillEvent.IsComplete,
				)
			}
		}

	case *pb.EngineMessage_DefinePoolAcknowledgement:
		// DefinePool acknowledgements are logged in the response list but don't affect pool tracker state
	}
}

func (wc *WebClient) Close() {
	if wc.matchingSendChan != nil {
		close(wc.matchingSendChan)
	}
	if wc.matchingStream != nil {
		wc.matchingStream.CloseSend()
	}
	if wc.conn != nil {
		wc.conn.Close()
	}
}

// checkServiceReachable tests TCP reachability of a host:port with a short timeout.
func checkServiceReachable(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// handleHealthCheck returns TCP reachability of the matching server
func (wc *WebClient) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	host := wc.serverHost
	serverPort := wc.serverPort
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"matching_server": map[string]interface{}{
			"reachable": checkServiceReachable(host, serverPort),
			"port":      serverPort,
		},
	})
}

// Connect establishes a connection to the matching server.
func (wc *WebClient) Connect() error {
	wc.mu.Lock()

	wc.streamGeneration++
	currentGen := wc.streamGeneration
	wc.tradeStreamActive = false

	// Capture old resources
	oldMatchingSendChan := wc.matchingSendChan
	oldMatchingStream := wc.matchingStream
	oldConn := wc.conn

	// Clear references
	wc.matchingSendChan = nil
	wc.matchingStream = nil
	wc.matchingClient = nil
	wc.conn = nil

	// Clear state
	wc.requests = make([]string, 0)
	wc.responses = make([]string, 0)
	wc.poolTracker = tc.NewPoolTracker()
	wc.pendingOrders = make(map[uint64]*tc.PendingOrderInfo)
	wc.globalSequenceNumber = 0
	wc.globalClientOrderId = 1000

	serverHost := wc.serverHost
	serverPort := wc.serverPort

	wc.mu.Unlock()

	// Close old resources outside the lock
	if oldMatchingSendChan != nil {
		close(oldMatchingSendChan)
	}
	if oldMatchingStream != nil {
		oldMatchingStream.CloseSend()
	}
	if oldConn != nil {
		oldConn.Close()
	}

	time.Sleep(100 * time.Millisecond)

	serverAddr := fmt.Sprintf("%s:%s", serverHost, serverPort)
	if !checkServiceReachable(serverHost, serverPort) {
		log.Printf("Matching server at %s is not reachable", serverAddr)
		return fmt.Errorf("matching server at %s is not reachable", serverAddr)
	}

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", serverAddr, err)
		return fmt.Errorf("failed to connect to %s: %v", serverAddr, err)
	}

	client := pb.NewMatchingServerServiceClient(conn)
	stream, err := client.CreateTradeStream(context.Background())
	if err != nil {
		conn.Close()
		log.Printf("Failed to create trade stream on %s: %v", serverAddr, err)
		return fmt.Errorf("failed to create trade stream: %v", err)
	}

	wc.mu.Lock()
	wc.conn = conn
	wc.matchingClient = client
	wc.matchingStream = stream
	wc.matchingSendChan = make(chan *pb.GatewayMessage, 10)
	wc.tradeStreamActive = true
	sendChan := wc.matchingSendChan
	wc.mu.Unlock()

	go func() {
		for msg := range sendChan {
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message: %v", err)
				wc.mu.Lock()
				if wc.streamGeneration == currentGen {
					wc.tradeStreamActive = false
				}
				wc.mu.Unlock()
				return
			}
		}
	}()

	go func() {
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				log.Println("Trade stream closed by server")
				wc.mu.Lock()
				if wc.streamGeneration == currentGen {
					wc.tradeStreamActive = false
				}
				wc.mu.Unlock()
				return
			}
			if err != nil {
				log.Printf("Error receiving response: %v", err)
				wc.mu.Lock()
				if wc.streamGeneration == currentGen {
					wc.tradeStreamActive = false
				}
				wc.mu.Unlock()
				return
			}

			wc.mu.Lock()
			if wc.streamGeneration == currentGen {
				respJSON, _ := marshaler.Marshal(resp)
				wc.responses = append(wc.responses, string(respJSON))
				wc.processEngineMessage(resp)
			}
			wc.mu.Unlock()
		}
	}()

	log.Printf("Connected to matching server at %s", serverAddr)
	return nil
}

// handleReconnect handles POST requests to reconnect to the matching server
func (wc *WebClient) handleReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"status": "error", "message": "Method not allowed"}`))
		return
	}

	connectErr := wc.Connect()

	wc.mu.RLock()
	active := wc.tradeStreamActive
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"streamActive": active,
	}
	if connectErr != nil {
		resp["error"] = connectErr.Error()
	} else {
		resp["status"] = "connected"
	}
	json.NewEncoder(w).Encode(resp)
}

// Send order handler
func (wc *WebClient) handleSendOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"status": "error", "message": "Method not allowed"}`))
		return
	}

	wc.mu.RLock()
	active := wc.tradeStreamActive
	wc.mu.RUnlock()

	if !active {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status": "error", "message": "Trade stream is not active"}`))
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status": "error", "message": "Failed to parse form"}`))
		return
	}

	messageType := r.FormValue("messageType")
	wc.sendMatchingOrder(w, r, messageType)
}

// sendMatchingOrder builds and sends a matching server proto message
func (wc *WebClient) sendMatchingOrder(w http.ResponseWriter, r *http.Request, messageType string) {
	seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
	marshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}

	var gatewayMsg *pb.GatewayMessage

	switch messageType {
	case "NewOrder":
		clientOrderId, _ := strconv.ParseUint(r.FormValue("clientOrderId"), 10, 64)
		trackerLegs, _ := tc.ParseLegsFromForm(r.FormValue("legSecurityIds"), r.FormValue("isOvers"))

		var slateIDProto *common.UUID
		if v := r.FormValue("slateId"); v != "" {
			if parsed, err := utils.ParseUUID(v); err == nil {
				slateIDProto = &common.UUID{Upper: parsed.Upper(), Lower: parsed.Lower()}
			}
		}
		lineupIndex, _ := strconv.ParseUint(r.FormValue("lineupIndex"), 10, 64)

		orderType := common.OrderType_LIMIT
		if r.FormValue("orderType") == "MARKET" {
			orderType = common.OrderType_MARKET
		}
		portion, _ := strconv.ParseUint(r.FormValue("portion"), 10, 64)
		quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 64)

		newOrderBody := &pb.NewOrder_Body{
			ClientOrderId: &common.UUID{Upper: 0, Lower: clientOrderId},
			SlateId:       slateIDProto,
			LineupIndex:   lineupIndex,
			OrderType:     orderType,
			Portion:       portion,
			Quantity:      quantity,
		}
		if v := r.FormValue("selfMatchId"); v != "" {
			if selfMatchID, err := utils.ParseUUID(v); err == nil {
				newOrderBody.SelfMatchId = &common.UUID{
					Upper: selfMatchID.Upper(),
					Lower: selfMatchID.Lower(),
				}
			}
		}

		gatewayMsg = &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Msg: &pb.GatewayMessage_NewOrder{
				NewOrder: &pb.NewOrder{Body: newOrderBody},
			},
		}

		wc.mu.Lock()
		wc.pendingOrders[clientOrderId] = &tc.PendingOrderInfo{
			Legs:        trackerLegs,
			SlateID:     r.FormValue("slateId"),
			LineupIndex: lineupIndex,
			Portion:     portion,
			Quantity:    quantity,
		}
		wc.mu.Unlock()

	case "CancelOrder":
		orderId, _ := strconv.ParseUint(r.FormValue("orderId"), 10, 64)
		gatewayMsg = &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Msg: &pb.GatewayMessage_CancelOrder{
				CancelOrder: &pb.CancelOrder{
					Body: &pb.CancelOrder_Body{OrderId: orderId},
				},
			},
		}

	case "DefinePool":
		var slateIDProto *common.UUID
		if v := r.FormValue("dpSlateId"); v != "" {
			if parsed, err := utils.ParseUUID(v); err == nil {
				slateIDProto = &common.UUID{Upper: parsed.Upper(), Lower: parsed.Lower()}
			}
		}
		totalUnits, _ := strconv.ParseUint(r.FormValue("dpTotalUnits"), 10, 64)
		numLineups, _ := strconv.ParseUint(r.FormValue("dpNumLineups"), 10, 64)

		gatewayMsg = &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Msg: &pb.GatewayMessage_DefinePool{
				DefinePool: &pb.DefinePool{
					Body: &pb.DefinePool_Body{
						SlateId:    slateIDProto,
						TotalUnits: totalUnits,
						NumLineups: numLineups,
					},
				},
			},
		}

		// Track pool definition for visualization
		if slateIDProto != nil {
			slateID := utils.UUIDFromUint64(slateIDProto.Upper, slateIDProto.Lower)
			wc.poolTracker.DefinePool(slateID.String(), totalUnits, numLineups)
		}

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status": "error", "message": "Invalid message type"}`))
		return
	}

	reqJSON, _ := marshaler.Marshal(gatewayMsg)
	wc.mu.Lock()
	wc.requests = append(wc.requests, string(reqJSON))
	wc.mu.Unlock()

	select {
	case wc.matchingSendChan <- gatewayMsg:
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "message sent"}`))
	case <-time.After(1 * time.Second):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestTimeout)
		w.Write([]byte(`{"status": "error", "message": "Timeout sending message"}`))
	}
}

// Get requests as JSON
func (wc *WebClient) handleRequests(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	requests := wc.requests
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// Get responses as JSON
func (wc *WebClient) handleResponses(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	responses := wc.responses
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// Global sequence number handler - returns current value and optionally increments or sets
func (wc *WebClient) handleSequenceNumber(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		wc.mu.Lock()
		wc.globalSequenceNumber++
		newSeq := wc.globalSequenceNumber
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"sequenceNumber": newSeq})
		return
	}

	if r.Method == "PUT" {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse form"})
			return
		}

		seqNum, err := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid sequence number"})
			return
		}

		wc.mu.Lock()
		wc.globalSequenceNumber = seqNum
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"sequenceNumber": seqNum})
		return
	}

	// GET: just return current value
	wc.mu.RLock()
	currentSeq := wc.globalSequenceNumber
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]uint64{"sequenceNumber": currentSeq})
}

// Global client order ID handler - returns current value and optionally increments or sets
func (wc *WebClient) handleClientOrderId(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		wc.mu.Lock()
		wc.globalClientOrderId++
		newId := wc.globalClientOrderId
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"clientOrderId": newId})
		return
	}

	if r.Method == "PUT" {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse form"})
			return
		}

		clientOrderId, err := strconv.ParseUint(r.FormValue("clientOrderId"), 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid client order ID"})
			return
		}

		wc.mu.Lock()
		wc.globalClientOrderId = clientOrderId
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"clientOrderId": clientOrderId})
		return
	}

	// GET: just return current value
	wc.mu.RLock()
	currentId := wc.globalClientOrderId
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]uint64{"clientOrderId": currentId})
}

// Get entry pools data as JSON
func (wc *WebClient) handleEntryPoolsData(w http.ResponseWriter, r *http.Request) {
	pools := wc.poolTracker.GetAllPoolsDisplay()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

// Trade page handler - displays the matching engine tester interface
func (wc *WebClient) handleTrade(w http.ResponseWriter, r *http.Request) {
	renderMainPage(w)
}

func renderMainPage(w http.ResponseWriter) {
	t, err := template.New("main").Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, nil)
}


func main() {
	cfg, err := tc.LoadConfig()
	if err != nil {
		tc.Fatal("Failed to load configuration: %v\nPlease ensure .env file exists with required fields (SERVER_HOST, SERVER_PORT)", err)
	}

	log.Printf("Starting Matching Engine Tester")

	client := NewWebClient(cfg)
	defer client.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/trade", http.StatusSeeOther)
	})
	http.HandleFunc("/trade", client.handleTrade)
	http.HandleFunc("/send", client.handleSendOrder)
	http.HandleFunc("/requests", client.handleRequests)
	http.HandleFunc("/responses", client.handleResponses)
	http.HandleFunc("/sequence-number", client.handleSequenceNumber)
	http.HandleFunc("/client-order-id", client.handleClientOrderId)
	http.HandleFunc("/entrypools-data", client.handleEntryPoolsData)
	http.HandleFunc("/reconnect", client.handleReconnect)
	http.HandleFunc("/health-check", client.handleHealthCheck)

	webAddr := fmt.Sprintf(":%s", cfg.WebPort)
	log.Printf("Starting web server on %s", webAddr)
	log.Fatal(http.ListenAndServe(webAddr, nil))
}
