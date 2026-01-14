package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	common "matching-clients/src/gen"
	pb "matching-clients/src/gen/matching"

	"github.com/openbook/shared/envloader"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type WebClient struct {
	conn   *grpc.ClientConn
	client pb.MatchingServerServiceClient
	mu     sync.RWMutex

	// Unified request/response storage
	requests  []string
	responses []string

	// Unified trade stream
	tradeStream       pb.MatchingServerService_CreateTradeStreamClient
	tradeSendChan     chan *pb.GatewayMessage
	tradeStreamActive bool
	streamGeneration  uint64 // Incremented on each reconnect to invalidate old goroutines

	// Global sequence number shared across all message types
	globalSequenceNumber uint64

	// Global client order ID counter
	globalClientOrderId uint64

	// Pool state tracking
	poolTracker   *PoolTracker
	pendingOrders map[uint64]*pb.NewOrder // map[clientOrderId]NewOrder

	// Target mode and connection info
	targetMode  string // "matching_server" or "gateway"
	serverHost  string
	serverPort  string
	gatewayPort string
}

func NewWebClient(cfg *Config) (*WebClient, error) {
	// Default to gateway
	targetMode := "gateway"
	targetPort := cfg.GatewayPort

	serverAddr := fmt.Sprintf("%s:%s", cfg.ServerHost, targetPort)

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", serverAddr, err)
	}

	wc := &WebClient{
		conn:                conn,
		client:              pb.NewMatchingServerServiceClient(conn),
		requests:            make([]string, 0),
		responses:           make([]string, 0),
		tradeSendChan:       make(chan *pb.GatewayMessage, 10),
		tradeStreamActive:   false,
		globalClientOrderId: 1000, // Start at 1001 (will increment on first use)
		poolTracker:         NewPoolTracker(),
		pendingOrders:       make(map[uint64]*pb.NewOrder),
		targetMode:          targetMode,
		serverHost:          cfg.ServerHost,
		serverPort:          cfg.ServerPort,
		gatewayPort:         cfg.GatewayPort,
	}

	// Initialize unified trade stream
	if err := wc.initTradeStream(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize trade stream: %v", err)
	}

	return wc, nil
}

func (wc *WebClient) initTradeStream() error {
	stream, err := wc.client.CreateTradeStream(context.Background())
	if err != nil {
		return err
	}

	wc.tradeStream = stream
	wc.tradeStreamActive = true

	// Goroutine to send messages from the channel
	go func() {
		for msg := range wc.tradeSendChan {
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message: %v", err)
				wc.mu.Lock()
				wc.tradeStreamActive = false
				wc.mu.Unlock()
				return
			}
		}
	}()

	// Goroutine to receive responses
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
				wc.tradeStreamActive = false
				wc.mu.Unlock()
				return
			}
			if err != nil {
				log.Printf("Error receiving response: %v", err)
				wc.mu.Lock()
				wc.tradeStreamActive = false
				wc.mu.Unlock()
				return
			}

			respJSON, _ := marshaler.Marshal(resp)
			wc.mu.Lock()
			wc.responses = append(wc.responses, string(respJSON))

			// Process the response for pool tracking
			wc.processEngineMessage(resp)

			wc.mu.Unlock()
		}
	}()

	return nil
}

// processEngineMessage processes engine responses and updates pool state
// NOTE: This method assumes wc.mu is already locked by the caller
func (wc *WebClient) processEngineMessage(resp *pb.EngineMessage) {
	switch event := resp.Event.(type) {
	case *pb.EngineMessage_NewOrderAcknowledgement:
		// Handle new order acknowledgement
		ack := event.NewOrderAcknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			clientOrderID := ack.Body.ClientOrderId
			orderID := ack.Body.OrderId
			var sequenceNumber uint64
			if resp.SequencedMessageBase != nil {
				sequenceNumber = resp.SequencedMessageBase.SequenceNumber
			}

			// Find the pending order
			if pendingOrder, exists := wc.pendingOrders[clientOrderID]; exists {
				// Convert legs to our internal format
				legs := make([]Leg, len(pendingOrder.Body.Legs))
				for i, leg := range pendingOrder.Body.Legs {
					legs[i] = Leg{
						LegSecurityID: leg.LegSecurityId,
						IsOver:        leg.IsOver,
					}
				}

				// Add to pool tracker
				wc.poolTracker.AddOrder(
					orderID,
					clientOrderID,
					legs,
					pendingOrder.Body.Portion,
					pendingOrder.Body.Quantity,
					sequenceNumber,
				)

				// Remove from pending orders
				delete(wc.pendingOrders, clientOrderID)
			}
		}

	case *pb.EngineMessage_CancelOrderAcknowledgement:
		// Handle cancel acknowledgement
		ack := event.CancelOrderAcknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			orderID := ack.Body.OrderId
			wc.poolTracker.RemoveOrder(orderID)
		}

	case *pb.EngineMessage_Elimination:
		// Handle elimination (order removed by server)
		elim := event.Elimination
		if elim != nil && elim.Body != nil {
			wc.poolTracker.RemoveOrder(elim.Body.OrderId)
		}

	case *pb.EngineMessage_Match:
		// Handle match/fill event
		match := event.Match
		if match.Body != nil {
			for _, fillEvent := range match.Body.FillEvents {
				wc.poolTracker.UpdateFromFill(
					fillEvent.OrderId,
					match.Body.MatchedQuantity,
					fillEvent.IsComplete,
				)
			}
		}
	}
}

func (wc *WebClient) Close() {
	// Close the send channel to stop the sender goroutine
	close(wc.tradeSendChan)

	// Close the stream
	if wc.tradeStream != nil {
		wc.tradeStream.CloseSend()
	}

	if wc.conn != nil {
		wc.conn.Close()
	}
}

// SwitchTarget switches the connection to a different target (matching_server or gateway)
func (wc *WebClient) SwitchTarget(newMode string) error {
	if newMode != "matching_server" && newMode != "gateway" {
		return fmt.Errorf("invalid target mode: %s", newMode)
	}

	wc.mu.Lock()
	if wc.targetMode == newMode {
		wc.mu.Unlock()
		return nil // Already connected to this target
	}

	// Increment generation to invalidate old goroutines
	wc.streamGeneration++
	currentGen := wc.streamGeneration

	// Mark stream as inactive before closing
	wc.tradeStreamActive = false

	// Capture old resources to close
	oldSendChan := wc.tradeSendChan
	oldStream := wc.tradeStream
	oldConn := wc.conn

	// Clear references before releasing lock
	wc.tradeSendChan = nil
	wc.tradeStream = nil
	wc.conn = nil

	// Determine new port
	var targetPort string
	switch newMode {
	case "gateway":
		targetPort = wc.gatewayPort
	default:
		targetPort = wc.serverPort
	}
	serverHost := wc.serverHost

	wc.mu.Unlock()

	// Close old resources outside the lock
	if oldSendChan != nil {
		close(oldSendChan)
	}
	if oldStream != nil {
		oldStream.CloseSend()
	}
	if oldConn != nil {
		oldConn.Close()
	}

	// Give old goroutines a moment to see the closed connection
	time.Sleep(100 * time.Millisecond)

	// Create new connection (outside lock)
	serverAddr := fmt.Sprintf("%s:%s", serverHost, targetPort)

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", serverAddr, err)
	}

	client := pb.NewMatchingServerServiceClient(conn)

	// Create trade stream
	stream, err := client.CreateTradeStream(context.Background())
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create trade stream: %v", err)
	}

	// Now acquire lock to update state
	wc.mu.Lock()
	wc.conn = conn
	wc.client = client
	wc.targetMode = newMode
	wc.tradeSendChan = make(chan *pb.GatewayMessage, 10)
	wc.tradeStream = stream
	wc.tradeStreamActive = true

	// Clear state
	wc.requests = make([]string, 0)
	wc.responses = make([]string, 0)
	wc.poolTracker = NewPoolTracker()
	wc.pendingOrders = make(map[uint64]*pb.NewOrder)
	wc.globalSequenceNumber = 0
	wc.globalClientOrderId = 1000

	sendChan := wc.tradeSendChan // Capture for goroutine
	wc.mu.Unlock()

	// Start sender goroutine
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

	// Start receiver goroutine
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

	log.Printf("Switched to target: %s at %s", newMode, serverAddr)
	return nil
}

// handleTargetMode handles GET/POST for target mode
func (wc *WebClient) handleTargetMode(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse form"})
			return
		}

		newMode := r.FormValue("targetMode")
		if err := wc.SwitchTarget(newMode); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"targetMode": newMode, "status": "switched"})
		return
	}

	// GET: return current target mode
	wc.mu.RLock()
	currentMode := wc.targetMode
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"targetMode": currentMode})
}

// Send order handler
func (wc *WebClient) handleSendOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"status": "error", "message": "Method not allowed"}`))
		return
	}

	// Check if stream is active
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

		// Parse legs from form (legSecurityIds and isOvers as comma-separated)
		legIdsStr := r.FormValue("legSecurityIds")
		isOversStr := r.FormValue("isOvers")

		var legs []*pb.NewOrder_Body_Leg
		if legIdsStr != "" && isOversStr != "" {
			legIdStrs := splitAndTrim(legIdsStr, ",")
			isOverStrs := splitAndTrim(isOversStr, ",")

			for i := 0; i < len(legIdStrs) && i < len(isOverStrs); i++ {
				legId, _ := strconv.ParseUint(legIdStrs[i], 10, 64)
				isOver := isOverStrs[i] == "true" || isOverStrs[i] == "1"
				legs = append(legs, &pb.NewOrder_Body_Leg{
					LegSecurityId: legId,
					IsOver:        isOver,
				})
			}
		}

		orderType := common.OrderType_LIMIT
		if r.FormValue("orderType") == "MARKET" {
			orderType = common.OrderType_MARKET
		}
		portion, _ := strconv.ParseUint(r.FormValue("portion"), 10, 64)
		quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 64)

		// handle optional selfMatchId (pointer presence)
		var selfMatchIdPtr *uint64
		if v := r.FormValue("selfMatchId"); v != "" {
			parsed, _ := strconv.ParseUint(v, 10, 64)
			selfMatchIdPtr = &parsed
		}

		newOrder := &pb.NewOrder{
			Body: &pb.NewOrder_Body{
				ClientOrderId: clientOrderId,
				Legs:          legs,
				OrderType:     orderType,
				Portion:       portion,
				Quantity:      quantity,
				SelfMatchId:   selfMatchIdPtr,
			},
		}

		gatewayMsg = &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Msg: &pb.GatewayMessage_NewOrder{
				NewOrder: newOrder,
			},
		}

		// Track pending order for later matching with acknowledgement
		wc.mu.Lock()
		wc.pendingOrders[clientOrderId] = newOrder
		wc.mu.Unlock()

	case "CancelOrder":
		orderId, _ := strconv.ParseUint(r.FormValue("orderId"), 10, 64)

		cancelOrder := &pb.CancelOrder{
			Body: &pb.CancelOrder_Body{
				OrderId: orderId,
			},
		}

		gatewayMsg = &pb.GatewayMessage{
			SequencedMessageBase: &common.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Msg: &pb.GatewayMessage_CancelOrder{
				CancelOrder: cancelOrder,
			},
		}

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status": "error", "message": "Invalid message type"}`))
		return
	}

	// Store the request
	reqJSON, _ := marshaler.Marshal(gatewayMsg)
	wc.mu.Lock()
	wc.requests = append(wc.requests, string(reqJSON))
	wc.mu.Unlock()

	// Send to the channel (non-blocking with timeout)
	select {
	case wc.tradeSendChan <- gatewayMsg:
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
		// Increment and return new value
		wc.mu.Lock()
		wc.globalSequenceNumber++
		newSeq := wc.globalSequenceNumber
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"sequenceNumber": newSeq})
		return
	}

	if r.Method == "PUT" {
		// Set to specific value
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
		// Increment and return new value
		wc.mu.Lock()
		wc.globalClientOrderId++
		newId := wc.globalClientOrderId
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]uint64{"clientOrderId": newId})
		return
	}

	if r.Method == "PUT" {
		// Set to specific value
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

// Entry Pools handler - redirects to main page
func (wc *WebClient) handleEntryPools(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/trade", http.StatusSeeOther)
}

// Get entry pools data as JSON
func (wc *WebClient) handleEntryPoolsData(w http.ResponseWriter, r *http.Request) {
	pools := wc.poolTracker.GetAllPoolsDisplay()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

// Trade page handler - displays unified send/receive interface
func (wc *WebClient) handleTrade(w http.ResponseWriter, r *http.Request) {
	renderMainPage(w)
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range splitString(s, sep) {
		trimmed := trimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func renderMainPage(w http.ResponseWriter) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Matching Engine Tester</title>
    <style>
        * { box-sizing: border-box; }
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; height: 100vh; display: flex; flex-direction: column; background: #f5f5f5; }
        .header { background: #333; color: white; padding: 10px 20px; display: flex; justify-content: space-between; align-items: center; flex-shrink: 0; }
        .header h1 { margin: 0; font-size: 24px; }
        .header-right { display: flex; align-items: center; gap: 10px; }
        .target-selector { display: flex; align-items: center; gap: 8px; }
        .target-selector label { font-size: 14px; }
        .target-selector select { padding: 5px 10px; border-radius: 3px; border: none; }
        .target-indicator { padding: 5px 10px; border-radius: 3px; font-size: 12px; font-weight: bold; }
        .target-matching { background: #e74c3c; }
        .target-gateway { background: #3498db; }
        
        /* Main layout - two columns */
        .main-container { display: flex; flex: 1; overflow: hidden; }
        .left-panel { flex: 1; display: flex; flex-direction: column; border-right: 3px solid #ddd; overflow-y: auto; background: #fff; }
        .right-panel { flex: 1; overflow-y: auto; padding: 20px; background: #f5f5f5; }
        
        /* Trade panel styles */
        .form-panel { padding: 20px; border-bottom: 2px solid #ddd; background: #f8f9fa; }
        .form-group { margin-bottom: 15px; }
        .form-group > label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, select { padding: 8px; border: 1px solid #ddd; border-radius: 3px; }
        button { padding: 10px 20px; background: #28a745; color: white; border: none; border-radius: 3px; cursor: pointer; margin-right: 10px; }
        button:hover { background: #218838; }
        .status-message { padding: 10px; margin: 10px 0; border-radius: 3px; display: inline-block; }
        .status-success { background: #d4edda; border: 1px solid #c3e6cb; color: #155724; }
        .status-error { background: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
        
        /* Request/Response columns */
        .messages-container { display: flex; flex: 1; overflow: hidden; }
        .messages-column { flex: 1; padding: 15px; overflow-y: auto; }
        .messages-column h3 { margin-top: 0; border-bottom: 2px solid #007bff; padding-bottom: 10px; font-size: 16px; }
        .messages-column:first-child { border-right: 1px solid #ddd; }
        .message-box { background: #f8f9fa; padding: 10px; margin-bottom: 8px; border-radius: 3px; border: 1px solid #ddd; }
        pre { white-space: pre-wrap; word-wrap: break-word; margin: 0; font-size: 11px; }
        
        /* Form field toggles */
        .message-type-selector { margin-bottom: 15px; }
        .message-type-selector label { display: inline; margin-right: 20px; font-weight: normal; }
        .message-type-selector input[type="radio"] { margin-right: 5px; }
        .form-fields { display: none; }
        .form-fields.active { display: block; }
        
        /* Leg styles */
        .leg-item { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; padding: 10px; background: #fff; border: 1px solid #ddd; border-radius: 4px; }
        .leg-item label { margin: 0; font-weight: normal; }
        .leg-id-input { width: 80px; }
        .over-under-toggle { display: flex; gap: 0; }
        .over-under-toggle button { padding: 6px 12px; border: 1px solid #aaa; background: #d0d0d0; color: #333; cursor: pointer; transition: all 0.2s; }
        .over-under-toggle button:first-child { border-radius: 4px 0 0 4px; border-right: none; }
        .over-under-toggle button:last-child { border-radius: 0 4px 4px 0; }
        .over-under-toggle button.active { background: #007bff; color: white; border-color: #007bff; }
        .over-under-toggle button:hover:not(.active) { background: #bbb; }
        .remove-leg-btn { padding: 6px 10px; background: #dc3545; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
        .remove-leg-btn:hover { background: #c82333; }
        .add-leg-btn { padding: 8px 16px; background: #6c757d; color: white; border: none; border-radius: 4px; cursor: pointer; margin-top: 5px; }
        .add-leg-btn:hover { background: #5a6268; }
        
        /* Entry Pools styles */
        .section-title { font-size: 18px; font-weight: bold; margin-bottom: 15px; padding-bottom: 10px; border-bottom: 2px solid #007bff; }
        .pool-card { background: white; border-radius: 8px; padding: 12px; margin-bottom: 12px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .pool-header { font-size: 14px; font-weight: bold; margin-bottom: 4px; color: #333; }
        .pool-info { font-size: 12px; color: #666; margin-bottom: 8px; }
        .lineups-container { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 5px; scroll-behavior: smooth; }
        .lineup-column { flex: 0 0 100px; background: #f8f9fa; border-radius: 4px; padding: 6px; }
        .lineup-header { font-weight: bold; margin-bottom: 2px; font-size: 11px; }
        .lineup-subheader { font-size: 9px; color: #666; margin-bottom: 6px; }
        .orders-stack { position: relative; height: 150px; background: #fff; border: 1px solid #ddd; border-radius: 3px; }
        .order-bar { position: absolute; left: 0; right: 0; cursor: pointer; transition: opacity 0.2s; }
        .order-bar:hover { opacity: 0.8; }
        .tooltip { position: fixed; background: #333; color: white; padding: 8px 12px; border-radius: 4px; font-size: 12px; white-space: nowrap; pointer-events: none; z-index: 1000; display: none; }
        .empty-lineup { height: 150px; display: flex; align-items: center; justify-content: center; color: #999; font-size: 11px; }
        .empty-pools { color: #999; font-style: italic; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Matching Engine Tester</h1>
        <div class="header-right">
            <div class="target-selector">
                <label>Target:</label>
                <select id="targetModeSelect">
                    <option value="matching_server">Matching Server (:50051)</option>
                    <option value="gateway">Gateway (:50052)</option>
                </select>
            </div>
            <span id="targetIndicator" class="target-indicator target-matching">MATCHING</span>
        </div>
    </div>
    
    <div class="main-container">
        <!-- Left Panel: Trade -->
        <div class="left-panel">
            <div class="form-panel">
                <h2 style="margin-top: 0; margin-bottom: 15px;">Send Order</h2>
                <form id="mainForm">
                    <div class="message-type-selector">
                        <label><input type="radio" name="messageType" value="NewOrder" checked> New Order</label>
                        <label><input type="radio" name="messageType" value="CancelOrder"> Cancel Order</label>
                    </div>

                    <div class="form-group">
                        <label>Sequence Number:</label>
                        <input type="number" name="sequenceNumber" value="0" required>
                    </div>

                    <!-- New Order Fields -->
                    <div id="newOrderFields" class="form-fields active">
                        <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 15px;">
                            <div class="form-group">
                                <label>Client Order ID:</label>
                                <input type="number" name="clientOrderId" value="1001">
                            </div>
                            <div class="form-group">
                                <label>Order Type:</label>
                                <select name="orderType">
                                    <option value="LIMIT">LIMIT</option>
                                    <option value="MARKET">MARKET</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label>Portion:</label>
                                <input type="number" name="portion" value="250000">
                            </div>
                            <div class="form-group">
                                <label>Quantity:</label>
                                <input type="number" name="quantity" value="5">
                            </div>
                            <div class="form-group">
                                <label>Self Match ID (optional):</label>
                                <input type="number" name="selfMatchId" placeholder="Optional">
                            </div>
                        </div>
                        
                        <!-- Legs Section -->
                        <div class="form-group" style="margin-top: 15px;">
                            <label>Legs:</label>
                            <div id="legsContainer"></div>
                            <button type="button" id="addLegBtn" class="add-leg-btn">+ Add Leg</button>
                        </div>
                        
                        <!-- Hidden inputs for form submission -->
                        <input type="hidden" name="legSecurityIds" id="legSecurityIdsHidden">
                        <input type="hidden" name="isOvers" id="isOversHidden">
                    </div>

                    <!-- Cancel Order Fields -->
                    <div id="cancelOrderFields" class="form-fields">
                        <div class="form-group">
                            <label>Order ID:</label>
                            <input type="number" name="orderId" style="width: 150px;">
                        </div>
                    </div>

                    <button type="submit">Send Message</button>
                    <div id="statusMessage" style="display: inline;"></div>
                </form>
            </div>
            
            <!-- Requests and Responses -->
            <div class="messages-container">
                <div class="messages-column">
                    <h3>Requests</h3>
                    <div id="requestList">
                        <p class="empty-pools">No requests yet.</p>
                    </div>
                </div>
                <div class="messages-column">
                    <h3>Responses</h3>
                    <div id="responseList">
                        <p class="empty-pools">No responses yet.</p>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Right Panel: Entry Pools -->
        <div class="right-panel">
            <div class="section-title">Entry Pools</div>
            <div id="poolsContainer">
                <p class="empty-pools">No pools yet. Submit orders to create entry pools.</p>
            </div>
        </div>
    </div>
    
    <div id="tooltip" class="tooltip"></div>

    <script>
        // ============ DOM Elements ============
        const form = document.getElementById('mainForm');
        const statusMessage = document.getElementById('statusMessage');
        const requestList = document.getElementById('requestList');
        const responseList = document.getElementById('responseList');
        const newOrderFields = document.getElementById('newOrderFields');
        const cancelOrderFields = document.getElementById('cancelOrderFields');
        const messageTypeRadios = document.querySelectorAll('input[name="messageType"]');
        const poolsContainer = document.getElementById('poolsContainer');

        // ============ Leg Management ============
        const legsContainer = document.getElementById('legsContainer');
        const addLegBtn = document.getElementById('addLegBtn');
        const legSecurityIdsHidden = document.getElementById('legSecurityIdsHidden');
        const isOversHidden = document.getElementById('isOversHidden');
        
        let legs = [
            { id: 101, isOver: false },
            { id: 102, isOver: true }
        ];
        
        function getNextLegId() {
            if (legs.length === 0) return 101;
            return Math.max(...legs.map(l => l.id)) + 1;
        }
        
        function renderLegs() {
            legsContainer.innerHTML = '';
            legs.forEach((leg, index) => {
                const legItem = document.createElement('div');
                legItem.className = 'leg-item';
                legItem.innerHTML = 
                    '<label>Leg ID:</label>' +
                    '<input type="number" class="leg-id-input" value="' + leg.id + '" data-index="' + index + '">' +
                    '<div class="over-under-toggle">' +
                        '<button type="button" class="over-btn ' + (!leg.isOver ? 'active' : '') + '" data-index="' + index + '">Under</button>' +
                        '<button type="button" class="under-btn ' + (leg.isOver ? 'active' : '') + '" data-index="' + index + '">Over</button>' +
                    '</div>' +
                    '<button type="button" class="remove-leg-btn" data-index="' + index + '">✕</button>';
                legsContainer.appendChild(legItem);
            });
            updateHiddenInputs();
            attachLegEventListeners();
        }
        
        function updateHiddenInputs() {
            legSecurityIdsHidden.value = legs.map(l => l.id).join(',');
            isOversHidden.value = legs.map(l => l.isOver.toString()).join(',');
        }
        
        function attachLegEventListeners() {
            document.querySelectorAll('.leg-id-input').forEach(input => {
                input.addEventListener('change', function() {
                    const index = parseInt(this.dataset.index);
                    legs[index].id = parseInt(this.value) || 0;
                    updateHiddenInputs();
                });
            });
            
            document.querySelectorAll('.under-btn').forEach(btn => {
                btn.addEventListener('click', function() {
                    const index = parseInt(this.dataset.index);
                    legs[index].isOver = true;
                    renderLegs();
                });
            });
            
            document.querySelectorAll('.over-btn').forEach(btn => {
                btn.addEventListener('click', function() {
                    const index = parseInt(this.dataset.index);
                    legs[index].isOver = false;
                    renderLegs();
                });
            });
            
            document.querySelectorAll('.remove-leg-btn').forEach(btn => {
                btn.addEventListener('click', function() {
                    const index = parseInt(this.dataset.index);
                    legs.splice(index, 1);
                    renderLegs();
                });
            });
        }
        
        addLegBtn.addEventListener('click', function() {
            legs.push({ id: getNextLegId(), isOver: false });
            renderLegs();
        });
        
        renderLegs();

        // ============ Form Type Toggle ============
        messageTypeRadios.forEach(radio => {
            radio.addEventListener('change', function() {
                if (this.value === 'NewOrder') {
                    newOrderFields.classList.add('active');
                    cancelOrderFields.classList.remove('active');
                } else {
                    newOrderFields.classList.remove('active');
                    cancelOrderFields.classList.add('active');
                }
            });
        });

        // ============ Sequence Number ============
        async function incrementSequenceNumber() {
            const seqInput = form.querySelector('[name="sequenceNumber"]');
            if (seqInput) {
                try {
                    const response = await fetch('/sequence-number', { method: 'POST' });
                    const data = await response.json();
                    seqInput.value = data.sequenceNumber;
                } catch (err) {
                    console.error('Error incrementing sequence number:', err);
                }
            }
        }

        async function syncSequenceNumberToServer() {
            const seqInput = form.querySelector('[name="sequenceNumber"]');
            if (seqInput) {
                const manualValue = parseInt(seqInput.value, 10);
                if (!isNaN(manualValue)) {
                    try {
                        await fetch('/sequence-number', {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                            body: 'sequenceNumber=' + manualValue
                        });
                    } catch (err) {
                        console.error('Error setting sequence number:', err);
                    }
                }
            }
        }

        async function loadInitialSequenceNumber() {
            const seqInput = form.querySelector('[name="sequenceNumber"]');
            if (seqInput) {
                try {
                    const response = await fetch('/sequence-number');
                    const data = await response.json();
                    seqInput.value = data.sequenceNumber;
                } catch (err) {
                    console.error('Error fetching sequence number:', err);
                }
            }
        }

        // ============ Client Order ID ============
        async function incrementClientOrderId() {
            const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
            if (clientOrderIdInput) {
                try {
                    const response = await fetch('/client-order-id', { method: 'POST' });
                    const data = await response.json();
                    clientOrderIdInput.value = data.clientOrderId;
                } catch (err) {
                    console.error('Error incrementing client order ID:', err);
                }
            }
        }

        async function syncClientOrderIdToServer() {
            const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
            if (clientOrderIdInput) {
                const manualValue = parseInt(clientOrderIdInput.value, 10);
                if (!isNaN(manualValue)) {
                    try {
                        await fetch('/client-order-id', {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                            body: 'clientOrderId=' + manualValue
                        });
                    } catch (err) {
                        console.error('Error setting client order ID:', err);
                    }
                }
            }
        }

        async function loadInitialClientOrderId() {
            const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
            if (clientOrderIdInput) {
                try {
                    const response = await fetch('/client-order-id');
                    const data = await response.json();
                    clientOrderIdInput.value = data.clientOrderId;
                } catch (err) {
                    console.error('Error fetching client order ID:', err);
                }
            }
        }

        loadInitialSequenceNumber();
        loadInitialClientOrderId();

        const seqInput = form.querySelector('[name="sequenceNumber"]');
        if (seqInput) seqInput.addEventListener('change', syncSequenceNumberToServer);
        
        const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
        if (clientOrderIdInput) clientOrderIdInput.addEventListener('change', syncClientOrderIdToServer);

        // ============ Form Submission ============
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            const formData = new FormData(form);
            const urlEncoded = new URLSearchParams(formData);
            const messageType = formData.get('messageType');

            try {
                const response = await fetch('/send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: urlEncoded
                });

                const data = await response.json();

                if (data.status === 'success') {
                    statusMessage.innerHTML = '<span class="status-message status-success">' + data.message + '</span>';
                    await incrementSequenceNumber();
                    if (messageType === 'NewOrder') {
                        await incrementClientOrderId();
                    }
                } else {
                    statusMessage.innerHTML = '<span class="status-message status-error">Error: ' + data.message + '</span>';
                }

                setTimeout(() => { statusMessage.innerHTML = ''; }, 3000);
            } catch (err) {
                statusMessage.innerHTML = '<span class="status-message status-error">Network error: ' + err.message + '</span>';
            }
        });

        // ============ Entry Pools ============
        const colors = [
            '#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8',
            '#F7DC6F', '#BB8FCE', '#85C1E2', '#F8B88B', '#AAB7B8',
            '#52BE80', '#F1948A', '#85929E', '#F39C12', '#8E44AD',
            '#3498DB', '#E74C3C', '#1ABC9C', '#2ECC71', '#E67E22'
        ];

        let colorIndex = 0;
        const orderColors = new Map();
        let lastPoolsJson = '';

        function getOrderColor(orderId) {
            if (!orderColors.has(orderId)) {
                orderColors.set(orderId, colors[colorIndex % colors.length]);
                colorIndex++;
            }
            return orderColors.get(orderId);
        }

        function formatNumber(n) {
            return n.toLocaleString();
        }

        function renderPools(pools, forceRender = false) {
            const currentJson = JSON.stringify(pools);
            if (!forceRender && currentJson === lastPoolsJson) {
                return; // No changes, skip re-render to preserve scroll
            }
            lastPoolsJson = currentJson;

            if (!pools || pools.length === 0) {
                poolsContainer.innerHTML = '<p class="empty-pools">No pools yet. Submit orders to create entry pools.</p>';
                return;
            }

            // Save scroll positions
            const scrollPositions = {};
            document.querySelectorAll('.lineups-container').forEach((container, idx) => {
                scrollPositions[idx] = container.scrollLeft;
            });

            let html = '';
            for (const pool of pools) {
                html += '<div class="pool-card">';
                html += '<div class="pool-header">Pool [' + pool.LegSecurityIDs.join(', ') + ']</div>';
                html += '<div class="pool-info">' + pool.NumLegs + ' legs, ' + formatNumber(pool.TotalUnits) + ' total units</div>';
                html += '<div class="lineups-container">';

                for (const lineup of pool.Lineups) {
                    html += '<div class="lineup-column">';
                    html += '<div class="lineup-header">' + lineup.LineupIndex + '</div>';

                    let ouStr = lineup.OverUnders.map(ou =>
                        (ou.IsOver ? 'O' : 'U')
                    ).join('');
                    html += '<div class="lineup-subheader">[' + ouStr + ']</div>';

                    if (!lineup.Orders || lineup.Orders.length === 0) {
                        html += '<div class="empty-lineup">Empty</div>';
                    } else {
                        html += '<div class="orders-stack" id="lineup-' + pool.PoolKey + '-' + lineup.LineupIndex + '"></div>';
                    }

                    html += '</div>';
                }

                html += '</div></div>';
            }

            poolsContainer.innerHTML = html;

            // Restore scroll positions
            document.querySelectorAll('.lineups-container').forEach((container, idx) => {
                if (scrollPositions[idx] !== undefined) {
                    container.scrollLeft = scrollPositions[idx];
                }
            });

            for (const pool of pools) {
                for (const lineup of pool.Lineups) {
                    if (lineup.Orders && lineup.Orders.length > 0) {
                        renderLineupOrders(pool.PoolKey, lineup.LineupIndex, lineup.Orders);
                    }
                }
            }
        }

        function renderLineupOrders(poolKey, lineupIndex, orders) {
            const container = document.getElementById('lineup-' + poolKey + '-' + lineupIndex);
            if (!container) return;

            const height = 150;
            const totalQuantity = orders.reduce((sum, o) => sum + o.RemainingQuantity, 0);

            let currentBottom = 0;

            for (let i = orders.length - 1; i >= 0; i--) {
                const order = orders[i];
                const barHeight = totalQuantity > 0 ? (order.RemainingQuantity / totalQuantity) * height : 0;

                const div = document.createElement('div');
                div.className = 'order-bar';
                div.style.bottom = currentBottom + 'px';
                div.style.height = Math.max(barHeight, 2) + 'px';
                div.style.backgroundColor = getOrderColor(order.OrderID);

                div.addEventListener('mouseenter', (e) => showTooltip(e, order));
                div.addEventListener('mousemove', updateTooltipPosition);
                div.addEventListener('mouseleave', hideTooltip);

                container.appendChild(div);
                currentBottom += barHeight;
            }
        }

        function showTooltip(e, order) {
            const tooltip = document.getElementById('tooltip');
            tooltip.innerHTML =
                'Order ID: ' + order.OrderID + '<br>' +
                'Client Order ID: ' + order.ClientOrderID + '<br>' +
                'Portion: ' + formatNumber(order.Portion) + '<br>' +
                'Remaining Qty: ' + formatNumber(order.RemainingQuantity) + '<br>' +
                'Original Qty: ' + formatNumber(order.OriginalQuantity);
            tooltip.style.display = 'block';
            updateTooltipPosition(e);
        }

        function updateTooltipPosition(e) {
            const tooltip = document.getElementById('tooltip');
            tooltip.style.left = (e.clientX + 10) + 'px';
            tooltip.style.top = (e.clientY + 10) + 'px';
        }

        function hideTooltip() {
            document.getElementById('tooltip').style.display = 'none';
        }

        async function fetchPools() {
            try {
                const response = await fetch('/entrypools-data');
                const pools = await response.json();
                renderPools(pools);
            } catch (err) {
                console.error('Error fetching pools:', err);
            }
        }

        // ============ Target Mode ============
        const targetModeSelect = document.getElementById('targetModeSelect');
        const targetIndicator = document.getElementById('targetIndicator');

        function updateTargetIndicator(mode) {
            if (mode === 'gateway') {
                targetIndicator.textContent = 'GATEWAY';
                targetIndicator.className = 'target-indicator target-gateway';
            } else {
                targetIndicator.textContent = 'RUST';
                targetIndicator.className = 'target-indicator target-matching';
            }
        }

        async function loadTargetMode() {
            try {
                const response = await fetch('/target-mode');
                const data = await response.json();
                targetModeSelect.value = data.targetMode;
                updateTargetIndicator(data.targetMode);
            } catch (err) {
                console.error('Error fetching target mode:', err);
            }
        }

        targetModeSelect.addEventListener('change', async function() {
            const newMode = this.value;
            try {
                const response = await fetch('/target-mode', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: 'targetMode=' + newMode
                });
                const data = await response.json();
                if (data.error) {
                    alert('Failed to switch target: ' + data.error);
                    loadTargetMode();
                } else {
                    updateTargetIndicator(newMode);
                    requestList.innerHTML = '<p class="empty-pools">No requests yet.</p>';
                    responseList.innerHTML = '<p class="empty-pools">No responses yet.</p>';
                    orderColors.clear();
                    colorIndex = 0;
                    loadInitialSequenceNumber();
                    loadInitialClientOrderId();
                }
            } catch (err) {
                console.error('Error switching target:', err);
                alert('Network error switching target');
                loadTargetMode();
            }
        });

        loadTargetMode();

        // ============ Polling ============
        setInterval(async () => {
            try {
                const reqResponse = await fetch('/requests');
                const requests = await reqResponse.json();

                if (requests && requests.length > 0) {
                    requestList.innerHTML = requests.slice().reverse().map(r =>
                        '<div class="message-box"><pre>' + r + '</pre></div>'
                    ).join('');
                } else {
                    requestList.innerHTML = '<p class="empty-pools">No requests yet.</p>';
                }

                const respResponse = await fetch('/responses');
                const responses = await respResponse.json();

                if (responses && responses.length > 0) {
                    responseList.innerHTML = responses.slice().reverse().map(r =>
                        '<div class="message-box"><pre>' + r + '</pre></div>'
                    ).join('');
                } else {
                    responseList.innerHTML = '<p class="empty-pools">No responses yet.</p>';
                }
            } catch (err) {
                console.error('Error fetching data:', err);
            }
        }, 500);

        fetchPools();
        setInterval(fetchPools, 500);
    </script>
</body>
</html>
`

	t, err := template.New("main").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, nil)
}

// fatal prints an error message to stderr and exits with code 1
func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// Config holds the configuration for the matching engine tester
type Config struct {
	ServerHost  string
	ServerPort  string
	GatewayPort string
	WebPort     string
}

// loadConfig loads and validates the configuration from .env file
func loadConfig() (*Config, error) {
	// Load .env file from current directory (required)
	if err := envloader.LoadEnvFile(""); err != nil {
		return nil, err
	}

	// Load required configuration
	serverHost, err := envloader.GetEnvAsString("SERVER_HOST", true)
	if err != nil {
		return nil, err
	}

	serverPort, err := envloader.GetEnvAsString("SERVER_PORT", true)
	if err != nil {
		return nil, err
	}

	// Load optional configuration with defaults
	webPort := envloader.GetEnvAsStringWithDefault("WEB_PORT", "8080")
	gatewayPort := envloader.GetEnvAsStringWithDefault("GATEWAY_PORT", "50052")

	return &Config{
		ServerHost:  serverHost,
		ServerPort:  serverPort,
		GatewayPort: gatewayPort,
		WebPort:     webPort,
	}, nil
}

func main() {
	// Load configuration from .env file
	cfg, err := loadConfig()
	if err != nil {
		fatal("Failed to load configuration: %v\nPlease ensure .env file exists with required fields (SERVER_HOST, SERVER_PORT)", err)
	}

	log.Printf("Initial target mode: gateway")

	client, err := NewWebClient(cfg)
	if err != nil {
		fatal("Failed to create client: %v", err)
	}
	defer client.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/trade", http.StatusSeeOther)
	})
	http.HandleFunc("/entrypools", client.handleEntryPools)
	http.HandleFunc("/entrypools-data", client.handleEntryPoolsData)
	http.HandleFunc("/trade", client.handleTrade)
	http.HandleFunc("/send", client.handleSendOrder)
	http.HandleFunc("/requests", client.handleRequests)
	http.HandleFunc("/responses", client.handleResponses)
	http.HandleFunc("/sequence-number", client.handleSequenceNumber)
	http.HandleFunc("/client-order-id", client.handleClientOrderId)
	http.HandleFunc("/target-mode", client.handleTargetMode)

	webAddr := fmt.Sprintf(":%s", cfg.WebPort)
	log.Printf("Starting web server on %s", webAddr)
	log.Fatal(http.ListenAndServe(webAddr, nil))
}
