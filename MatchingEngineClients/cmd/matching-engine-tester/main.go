package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	pb "CommandCenter/src/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type WebClient struct {
	conn   *grpc.ClientConn
	client pb.MatchingServerServiceClient
	mu     sync.RWMutex

	// Request storage
	heartbeatRequests   []string
	orderNewRequests    []string
	orderCancelRequests []string

	// Response storage
	heartbeatResponses   []string
	orderNewResponses    []string
	orderCancelResponses []string

	// Persistent heartbeat stream
	heartbeatStream       pb.MatchingServerService_CreateHeartbeatResponseStreamClient
	heartbeatSendChan     chan *pb.Heartbeat
	heartbeatStreamActive bool

	// Persistent order new stream
	orderNewStream       pb.MatchingServerService_CreateOrderNewResponseStreamClient
	orderNewSendChan     chan *pb.OrderNew
	orderNewStreamActive bool

	// Persistent order cancel stream
	orderCancelStream       pb.MatchingServerService_CreateOrderCancelResponseStreamClient
	orderCancelSendChan     chan *pb.OrderCancel
	orderCancelStreamActive bool

	// Global sequence number shared across all message types
	globalSequenceNumber uint64

	// Global client order ID counter
	globalClientOrderId uint64

	// Pool state tracking
	poolTracker   *PoolTracker
	pendingOrders map[uint64]*pb.OrderNew // map[clientOrderId]OrderNew
}

func NewWebClient(serverAddr string) (*WebClient, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	wc := &WebClient{
		conn:                    conn,
		client:                  pb.NewMatchingServerServiceClient(conn),
		heartbeatRequests:       make([]string, 0),
		orderNewRequests:        make([]string, 0),
		orderCancelRequests:     make([]string, 0),
		heartbeatResponses:      make([]string, 0),
		orderNewResponses:       make([]string, 0),
		orderCancelResponses:    make([]string, 0),
		heartbeatSendChan:       make(chan *pb.Heartbeat, 10),
		heartbeatStreamActive:   false,
		orderNewSendChan:        make(chan *pb.OrderNew, 10),
		orderNewStreamActive:    false,
		orderCancelSendChan:     make(chan *pb.OrderCancel, 10),
		orderCancelStreamActive: false,
		globalClientOrderId:     1000, // Start at 1001 (will increment on first use)
		poolTracker:             NewPoolTracker(),
		pendingOrders:           make(map[uint64]*pb.OrderNew),
	}

	// Initialize persistent streams
	if err := wc.initHeartbeatStream(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize heartbeat stream: %v", err)
	}

	if err := wc.initOrderNewStream(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize order new stream: %v", err)
	}

	if err := wc.initOrderCancelStream(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize order cancel stream: %v", err)
	}

	return wc, nil
}

func (wc *WebClient) initHeartbeatStream() error {
	stream, err := wc.client.CreateHeartbeatResponseStream(context.Background())
	if err != nil {
		return err
	}

	wc.heartbeatStream = stream
	wc.heartbeatStreamActive = true

	// Goroutine to send heartbeats from the channel
	go func() {
		for heartbeat := range wc.heartbeatSendChan {
			if err := stream.Send(heartbeat); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				wc.mu.Lock()
				wc.heartbeatStreamActive = false
				wc.mu.Unlock()
				return
			}
		}
	}()

	// Goroutine to receive heartbeat responses
	go func() {
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				log.Println("Heartbeat stream closed by server")
				wc.mu.Lock()
				wc.heartbeatStreamActive = false
				wc.mu.Unlock()
				return
			}
			if err != nil {
				log.Printf("Error receiving heartbeat response: %v", err)
				wc.mu.Lock()
				wc.heartbeatStreamActive = false
				wc.mu.Unlock()
				return
			}

			respJSON, _ := marshaler.Marshal(resp)
			wc.mu.Lock()
			wc.heartbeatResponses = append(wc.heartbeatResponses, string(respJSON))
			wc.mu.Unlock()
		}
	}()

	return nil
}

func (wc *WebClient) initOrderNewStream() error {
	stream, err := wc.client.CreateOrderNewResponseStream(context.Background())
	if err != nil {
		return err
	}

	wc.orderNewStream = stream
	wc.orderNewStreamActive = true

	// Goroutine to send orders from the channel
	go func() {
		for order := range wc.orderNewSendChan {
			if err := stream.Send(order); err != nil {
				log.Printf("Failed to send order: %v", err)
				wc.mu.Lock()
				wc.orderNewStreamActive = false
				wc.mu.Unlock()
				return
			}
		}
	}()

	// Goroutine to receive order responses
	go func() {
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				log.Println("Order new stream closed by server")
				wc.mu.Lock()
				wc.orderNewStreamActive = false
				wc.mu.Unlock()
				return
			}
			if err != nil {
				log.Printf("Error receiving order new response: %v", err)
				wc.mu.Lock()
				wc.orderNewStreamActive = false
				wc.mu.Unlock()
				return
			}

			respJSON, _ := marshaler.Marshal(resp)
			wc.mu.Lock()
			wc.orderNewResponses = append(wc.orderNewResponses, string(respJSON))

			// Process the response for pool tracking
			wc.processOrderNewResponse(resp)

			wc.mu.Unlock()
		}
	}()

	return nil
}

func (wc *WebClient) initOrderCancelStream() error {
	stream, err := wc.client.CreateOrderCancelResponseStream(context.Background())
	if err != nil {
		return err
	}

	wc.orderCancelStream = stream
	wc.orderCancelStreamActive = true

	// Goroutine to send cancels from the channel
	go func() {
		for cancel := range wc.orderCancelSendChan {
			if err := stream.Send(cancel); err != nil {
				log.Printf("Failed to send cancel: %v", err)
				wc.mu.Lock()
				wc.orderCancelStreamActive = false
				wc.mu.Unlock()
				return
			}
		}
	}()

	// Goroutine to receive cancel responses
	go func() {
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				log.Println("Order cancel stream closed by server")
				wc.mu.Lock()
				wc.orderCancelStreamActive = false
				wc.mu.Unlock()
				return
			}
			if err != nil {
				log.Printf("Error receiving order cancel response: %v", err)
				wc.mu.Lock()
				wc.orderCancelStreamActive = false
				wc.mu.Unlock()
				return
			}

			respJSON, _ := marshaler.Marshal(resp)
			wc.mu.Lock()
			wc.orderCancelResponses = append(wc.orderCancelResponses, string(respJSON))

			// Process the response for pool tracking
			wc.processOrderCancelResponse(resp)

			wc.mu.Unlock()
		}
	}()

	return nil
}

// processOrderNewResponse processes order new responses and updates pool state
// NOTE: This method assumes wc.mu is already locked by the caller
func (wc *WebClient) processOrderNewResponse(resp *pb.OrderNewResponseEnvelope) {
	switch contents := resp.Contents.(type) {
	case *pb.OrderNewResponseEnvelope_Acknowledgement:
		// Handle acknowledgement
		ack := contents.Acknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			clientOrderID := ack.Body.ClientOrderId
			orderID := ack.Body.OrderId
			sequenceNumber := ack.SequencedMessageBase.SequenceNumber

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

	case *pb.OrderNewResponseEnvelope_Fill:
		// Handle fill event
		fill := contents.Fill
		if fill.Body != nil {
			for _, fillEntry := range fill.Body.Fills {
				wc.poolTracker.UpdateFromFill(
					fillEntry.OrderId,
					fill.Body.MatchedQuantity,
					fillEntry.IsComplete,
				)
			}
		}

	case *pb.OrderNewResponseEnvelope_Elimination:
		// Handle elimination (order removed by server due to self-match prevention, etc.)
		elim := contents.Elimination
		if elim != nil && elim.Body != nil {
			wc.poolTracker.RemoveOrder(elim.Body.OrderId)
		}
	}
}

// processOrderCancelResponse processes order cancel responses and updates pool state
// NOTE: This method assumes wc.mu is already locked by the caller
func (wc *WebClient) processOrderCancelResponse(resp *pb.OrderCancelResponseEnvelope) {
	switch contents := resp.Contents.(type) {
	case *pb.OrderCancelResponseEnvelope_Acknowledgement:
		// Handle cancel acknowledgement
		ack := contents.Acknowledgement
		if ack.FallibleBase != nil && ack.FallibleBase.Success && ack.Body != nil {
			orderID := ack.Body.OrderId
			wc.poolTracker.RemoveOrder(orderID)
		}
	}
}

func (wc *WebClient) Close() {
	// Close all channels to stop the sender goroutines
	close(wc.heartbeatSendChan)
	close(wc.orderNewSendChan)
	close(wc.orderCancelSendChan)

	// Close all streams
	if wc.heartbeatStream != nil {
		wc.heartbeatStream.CloseSend()
	}
	if wc.orderNewStream != nil {
		wc.orderNewStream.CloseSend()
	}
	if wc.orderCancelStream != nil {
		wc.orderCancelStream.CloseSend()
	}

	if wc.conn != nil {
		wc.conn.Close()
	}
}

// Heartbeat handler
func (wc *WebClient) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Check if stream is active
		wc.mu.RLock()
		active := wc.heartbeatStreamActive
		wc.mu.RUnlock()

		if !active {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "error", "message": "Heartbeat stream is not active"}`))
			return
		}

		// Send heartbeat through the persistent stream
		heartbeat := &pb.Heartbeat{
			MessageBase: &pb.MessageBase{
				VersionMajor: uint32(pb.VersionMajor_VERSION_MAJOR_VALUE),
				VersionMinor: uint32(pb.VersionMinor_VERSION_MINOR_VALUE),
				MessageType:  pb.MessageType_HEARTBEAT,
			},
		}

		// Store the request
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		reqJSON, _ := marshaler.Marshal(heartbeat)
		wc.mu.Lock()
		wc.heartbeatRequests = append(wc.heartbeatRequests, string(reqJSON))
		wc.mu.Unlock()

		// Send to the channel (non-blocking with timeout)
		select {
		case wc.heartbeatSendChan <- heartbeat:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "success", "message": "heartbeat sent"}`))
		case <-time.After(1 * time.Second):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte(`{"status": "error", "message": "Timeout sending heartbeat"}`))
		}
		return
	}

	wc.mu.RLock()
	responses := wc.heartbeatResponses
	wc.mu.RUnlock()
	renderPage(w, "heartbeat", responses)
}

// Get heartbeat requests as JSON
func (wc *WebClient) handleHeartbeatRequests(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	requests := wc.heartbeatRequests
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// Get heartbeat responses as JSON
func (wc *WebClient) handleHeartbeatResponses(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	responses := wc.heartbeatResponses
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// Order New handler
func (wc *WebClient) handleOrderNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Check if stream is active
		wc.mu.RLock()
		active := wc.orderNewStreamActive
		wc.mu.RUnlock()

		if !active {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "error", "message": "Order new stream is not active"}`))
			return
		}

		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": "error", "message": "Failed to parse form"}`))
			return
		}

		seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		clientOrderId, _ := strconv.ParseUint(r.FormValue("clientOrderId"), 10, 64)

		// Parse legs from form (legSecurityIds and isOvers as comma-separated)
		legIdsStr := r.FormValue("legSecurityIds")
		isOversStr := r.FormValue("isOvers")

		var legs []*pb.OrderNew_Body_Leg
		if legIdsStr != "" && isOversStr != "" {
			legIdStrs := splitAndTrim(legIdsStr, ",")
			isOverStrs := splitAndTrim(isOversStr, ",")

			for i := 0; i < len(legIdStrs) && i < len(isOverStrs); i++ {
				legId, _ := strconv.ParseUint(legIdStrs[i], 10, 64)
				isOver := isOverStrs[i] == "true" || isOverStrs[i] == "1"
				legs = append(legs, &pb.OrderNew_Body_Leg{
					LegSecurityId: legId,
					IsOver:        isOver,
				})
			}
		}

		orderType := pb.OrderType_LIMIT
		if r.FormValue("orderType") == "MARKET" {
			orderType = pb.OrderType_MARKET
		}
		portion, _ := strconv.ParseUint(r.FormValue("portion"), 10, 64)
		quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 64)

		// handle optional selfMatchId (pointer presence)
		var selfMatchIdPtr *uint64
		if v := r.FormValue("selfMatchId"); v != "" {
			parsed, _ := strconv.ParseUint(v, 10, 64)
			selfMatchIdPtr = &parsed
		}

		order := &pb.OrderNew{
			MessageBase: &pb.MessageBase{
				VersionMajor: uint32(pb.VersionMajor_VERSION_MAJOR_VALUE),
				VersionMinor: uint32(pb.VersionMinor_VERSION_MINOR_VALUE),
				MessageType:  pb.MessageType_ORDER_NEW,
			},
			SequencedMessageBase: &pb.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Body: &pb.OrderNew_Body{
				ClientOrderId: clientOrderId,
				Legs:          legs,
				OrderType:     orderType,
				Portion:       portion,
				Quantity:      quantity,
				SelfMatchId:   selfMatchIdPtr,
			},
		}

		// Store the request
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		reqJSON, _ := marshaler.Marshal(order)
		wc.mu.Lock()
		wc.orderNewRequests = append(wc.orderNewRequests, string(reqJSON))
		// Track pending order for later matching with acknowledgement
		wc.pendingOrders[clientOrderId] = order
		wc.mu.Unlock()

		// Send to the channel (non-blocking with timeout)
		select {
		case wc.orderNewSendChan <- order:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "success", "message": "order sent"}`))
		case <-time.After(1 * time.Second):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte(`{"status": "error", "message": "Timeout sending order"}`))
		}
		return
	}

	wc.mu.RLock()
	responses := wc.orderNewResponses
	wc.mu.RUnlock()
	renderPage(w, "ordernew", responses)
}

// Get order new requests as JSON
func (wc *WebClient) handleOrderNewRequests(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	requests := wc.orderNewRequests
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// Get order new responses as JSON
func (wc *WebClient) handleOrderNewResponses(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	responses := wc.orderNewResponses
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// Order Cancel handler
func (wc *WebClient) handleOrderCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Check if stream is active
		wc.mu.RLock()
		active := wc.orderCancelStreamActive
		wc.mu.RUnlock()

		if !active {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "error", "message": "Order cancel stream is not active"}`))
			return
		}

		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": "error", "message": "Failed to parse form"}`))
			return
		}

		seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		orderId, _ := strconv.ParseUint(r.FormValue("orderId"), 10, 64)

		cancel := &pb.OrderCancel{
			MessageBase: &pb.MessageBase{
				VersionMajor: uint32(pb.VersionMajor_VERSION_MAJOR_VALUE),
				VersionMinor: uint32(pb.VersionMinor_VERSION_MINOR_VALUE),
				MessageType:  pb.MessageType_ORDER_CANCEL,
			},
			SequencedMessageBase: &pb.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			Body: &pb.OrderCancel_Body{
				OrderId: orderId,
			},
		}

		// Store the request
		marshaler := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}
		reqJSON, _ := marshaler.Marshal(cancel)
		wc.mu.Lock()
		wc.orderCancelRequests = append(wc.orderCancelRequests, string(reqJSON))
		wc.mu.Unlock()

		// Send to the channel (non-blocking with timeout)
		select {
		case wc.orderCancelSendChan <- cancel:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "success", "message": "cancel sent"}`))
		case <-time.After(1 * time.Second):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte(`{"status": "error", "message": "Timeout sending cancel"}`))
		}
		return
	}

	wc.mu.RLock()
	responses := wc.orderCancelResponses
	wc.mu.RUnlock()
	renderPage(w, "ordercancel", responses)
}

// Get order cancel requests as JSON
func (wc *WebClient) handleOrderCancelRequests(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	requests := wc.orderCancelRequests
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// Get order cancel responses as JSON
func (wc *WebClient) handleOrderCancelResponses(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	responses := wc.orderCancelResponses
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

// Entry Pools handler - displays all pool states
func (wc *WebClient) handleEntryPools(w http.ResponseWriter, r *http.Request) {
	renderEntryPoolsPage(w)
}

// Get entry pools data as JSON
func (wc *WebClient) handleEntryPoolsData(w http.ResponseWriter, r *http.Request) {
	pools := wc.poolTracker.GetAllPoolsDisplay()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
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

func renderEntryPoolsPage(w http.ResponseWriter) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Matching Engine Tester - Entry Pools</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; background: #f5f5f5; }
        .header { background: #333; color: white; padding: 10px 20px; }
        .header h1 { margin: 0; font-size: 24px; }
        .nav { background: #444; padding: 10px 20px; }
        .nav a { margin-right: 15px; padding: 5px 15px; text-decoration: none; background: #007bff; color: white; border-radius: 3px; display: inline-block; }
        .nav a:hover { background: #0056b3; }
        .container { padding: 20px; }
        .pool-card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .pool-header { font-size: 18px; font-weight: bold; margin-bottom: 5px; color: #333; }
        .pool-info { font-size: 14px; color: #666; margin-bottom: 15px; }
        .lineups-container { display: flex; gap: 15px; overflow-x: auto; }
        .lineup-column { flex: 0 0 200px; background: #f8f9fa; border-radius: 4px; padding: 10px; }
        .lineup-header { font-weight: bold; margin-bottom: 5px; font-size: 14px; }
        .lineup-subheader { font-size: 11px; color: #666; margin-bottom: 10px; }
        .orders-stack { position: relative; height: 300px; background: #fff; border: 1px solid #ddd; border-radius: 3px; }
        .order-bar { position: absolute; left: 0; right: 0; cursor: pointer; transition: opacity 0.2s; }
        .order-bar:hover { opacity: 0.8; }
        .tooltip { position: absolute; background: #333; color: white; padding: 8px 12px; border-radius: 4px; font-size: 12px; white-space: nowrap; pointer-events: none; z-index: 1000; display: none; }
        .empty-lineup { height: 300px; display: flex; align-items: center; justify-content: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Matching Engine Tester</h1>
    </div>
    <div class="nav">
        <a href="/entrypools">Entry Pools</a>
        <a href="/heartbeat">Heartbeat</a>
        <a href="/ordernew">Order New</a>
        <a href="/ordercancel">Order Cancel</a>
    </div>
    <div class="container" id="poolsContainer">
        <p>Loading pools...</p>
    </div>

    <div id="tooltip" class="tooltip"></div>

    <script>
        const colors = [
            '#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8',
            '#F7DC6F', '#BB8FCE', '#85C1E2', '#F8B88B', '#AAB7B8',
            '#52BE80', '#F1948A', '#85929E', '#F39C12', '#8E44AD',
            '#3498DB', '#E74C3C', '#1ABC9C', '#2ECC71', '#E67E22'
        ];

        let colorIndex = 0;
        const orderColors = new Map();

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

        function renderPools(pools) {
            const container = document.getElementById('poolsContainer');

            if (!pools || pools.length === 0) {
                container.innerHTML = '<p style="color: #999;">No pools yet. Submit orders to create entry pools.</p>';
                return;
            }

            let html = '';
            for (const pool of pools) {
                html += '<div class="pool-card">';
                html += '<div class="pool-header">Pool [' + pool.LegSecurityIDs.join(', ') + ']</div>';
                html += '<div class="pool-info">' + pool.NumLegs + ' legs, ' + formatNumber(pool.TotalUnits) + ' total units</div>';
                html += '<div class="lineups-container">';

                for (const lineup of pool.Lineups) {
                    html += '<div class="lineup-column">';
                    html += '<div class="lineup-header">Lineup ' + lineup.LineupIndex + '</div>';

                    // Show over/under combination
                    let ouStr = lineup.OverUnders.map(ou =>
                        ou.LegSecurityID + '=' + (ou.IsOver ? 'O' : 'U')
                    ).join(', ');
                    html += '<div class="lineup-subheader">[' + ouStr + ']</div>';

                    if (!lineup.Orders || lineup.Orders.length === 0) {
                        html += '<div class="empty-lineup">No orders</div>';
                    } else {
                        html += '<div class="orders-stack" id="lineup-' + pool.PoolKey + '-' + lineup.LineupIndex + '">';
                        html += '</div>';
                    }

                    html += '</div>';
                }

                html += '</div>';
                html += '</div>';
            }

            container.innerHTML = html;

            // Render order bars
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

            const height = 300;
            const totalQuantity = orders.reduce((sum, o) => sum + o.RemainingQuantity, 0);

            let currentBottom = 0;

            // Orders are already sorted by portion (high to low), then FIFO
            // Render from bottom to top (highest portion at top)
            for (let i = orders.length - 1; i >= 0; i--) {
                const order = orders[i];
                const barHeight = totalQuantity > 0 ? (order.RemainingQuantity / totalQuantity) * height : 0;

                const div = document.createElement('div');
                div.className = 'order-bar';
                div.style.bottom = currentBottom + 'px';
                div.style.height = Math.max(barHeight, 2) + 'px';
                div.style.backgroundColor = getOrderColor(order.OrderID);

                div.addEventListener('mouseenter', function(e) {
                    showTooltip(e, order);
                });
                div.addEventListener('mousemove', function(e) {
                    updateTooltipPosition(e);
                });
                div.addEventListener('mouseleave', function() {
                    hideTooltip();
                });

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
            tooltip.style.left = (e.pageX + 10) + 'px';
            tooltip.style.top = (e.pageY + 10) + 'px';
        }

        function hideTooltip() {
            const tooltip = document.getElementById('tooltip');
            tooltip.style.display = 'none';
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

        // Initial fetch
        fetchPools();

        // Auto-refresh every 500ms
        setInterval(fetchPools, 500);
    </script>
</body>
</html>
`

	t, err := template.New("entrypools").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, nil)
}

func renderPage(w http.ResponseWriter, pageType string, responses []string) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Matching Engine Tester - {{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; height: 100vh; display: flex; flex-direction: column; }
        .header { background: #333; color: white; padding: 10px 20px; }
        .header h1 { margin: 0; font-size: 24px; }
        .nav { background: #444; padding: 10px 20px; }
        .nav a { margin-right: 15px; padding: 5px 15px; text-decoration: none; background: #007bff; color: white; border-radius: 3px; display: inline-block; }
        .nav a:hover { background: #0056b3; }
        .form-panel { padding: 20px; border-bottom: 2px solid #ddd; background: #f8f9fa; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, select { padding: 8px; border: 1px solid #ddd; border-radius: 3px; }
        button { padding: 10px 20px; background: #28a745; color: white; border: none; border-radius: 3px; cursor: pointer; margin-right: 10px; }
        button:hover { background: #218838; }
        .status-message { padding: 10px; margin: 10px 0; border-radius: 3px; display: inline-block; }
        .status-success { background: #d4edda; border: 1px solid #c3e6cb; color: #155724; }
        .status-error { background: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
        .columns { display: flex; flex: 1; overflow: hidden; }
        .column { flex: 1; padding: 20px; overflow-y: auto; }
        .column h3 { margin-top: 0; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        .left-column { border-right: 2px solid #ddd; }
        .right-column { background: #f8f9fa; }
        .message-box { background: white; padding: 15px; margin-bottom: 10px; border-radius: 3px; border: 1px solid #ddd; }
        pre { white-space: pre-wrap; word-wrap: break-word; margin: 0; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Matching Engine Tester</h1>
    </div>
    <div class="nav">
        <a href="/entrypools">Entry Pools</a>
        <a href="/heartbeat">Heartbeat</a>
        <a href="/ordernew">Order New</a>
        <a href="/ordercancel">Order Cancel</a>
    </div>
    <div class="form-panel">
        {{if eq .Type "heartbeat"}}
        <h2>Send Heartbeat</h2>
        <form id="mainForm" style="display: inline;">
            <button type="submit">Send Heartbeat</button>
            <div id="statusMessage" style="display: inline;"></div>
        </form>
        {{end}}

        {{if eq .Type "ordernew"}}
        <h2>Send Order</h2>
        <form id="mainForm">
            <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 15px;">
                <div class="form-group">
                    <label>Sequence Number:</label>
                    <input type="number" name="sequenceNumber" value="0" required>
                </div>
                <div class="form-group">
                    <label>Client Order ID:</label>
                    <input type="number" name="clientOrderId" value="1001" required>
                </div>
                <div class="form-group">
                    <label>Leg Security IDs:</label>
                    <input type="text" name="legSecurityIds" placeholder="101,102" value="101,102" required>
                </div>
                <div class="form-group">
                    <label>Is Over (true/false):</label>
                    <input type="text" name="isOvers" placeholder="false,true" value="false,true" required>
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
                    <input type="number" name="portion" value="250000" required>
                </div>
                <div class="form-group">
                    <label>Quantity:</label>
                    <input type="number" name="quantity" value="5" required>
                </div>
                <div class="form-group">
                    <label>Self Match ID (optional):</label>
                    <input type="number" name="selfMatchId" placeholder="Optional">
                </div>
            </div>
            <button type="submit">Send Order</button>
            <div id="statusMessage" style="display: inline;"></div>
        </form>
        {{end}}

        {{if eq .Type "ordercancel"}}
        <h2>Cancel Order</h2>
        <form id="mainForm">
            <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 15px; max-width: 600px;">
                <div class="form-group">
                    <label>Sequence Number:</label>
                    <input type="number" name="sequenceNumber" value="0" required>
                </div>
                <div class="form-group">
                    <label>Order ID:</label>
                    <input type="number" name="orderId" required>
                </div>
            </div>
            <button type="submit">Cancel Order</button>
            <div id="statusMessage" style="display: inline;"></div>
        </form>
        {{end}}
    </div>

    <div class="columns">
        <div class="column left-column">
            <h3>Requests</h3>
            <div id="requestList">
                <p>No requests yet.</p>
            </div>
        </div>
        <div class="column right-column">
            <h3>Responses</h3>
            <div id="responseList">
                <p>No responses yet.</p>
            </div>
        </div>
    </div>

    <script>
        const pageType = '{{.Type}}';
        const form = document.getElementById('mainForm');
        const statusMessage = document.getElementById('statusMessage');
        const requestList = document.getElementById('requestList');
        const responseList = document.getElementById('responseList');

        // Increment the global sequence number on the server and update the field
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

        // Sync manually typed sequence number to server
        async function syncSequenceNumberToServer() {
            const seqInput = form.querySelector('[name="sequenceNumber"]');
            if (seqInput) {
                const manualValue = parseInt(seqInput.value, 10);
                if (!isNaN(manualValue)) {
                    try {
                        await fetch('/sequence-number', {
                            method: 'PUT',
                            headers: {
                                'Content-Type': 'application/x-www-form-urlencoded',
                            },
                            body: 'sequenceNumber=' + manualValue
                        });
                    } catch (err) {
                        console.error('Error setting sequence number:', err);
                    }
                }
            }
        }

        // Load current sequence number from server on page load only
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

        // Increment the global client order ID on the server and update the field
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

        // Sync manually typed client order ID to server
        async function syncClientOrderIdToServer() {
            const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
            if (clientOrderIdInput) {
                const manualValue = parseInt(clientOrderIdInput.value, 10);
                if (!isNaN(manualValue)) {
                    try {
                        await fetch('/client-order-id', {
                            method: 'PUT',
                            headers: {
                                'Content-Type': 'application/x-www-form-urlencoded',
                            },
                            body: 'clientOrderId=' + manualValue
                        });
                    } catch (err) {
                        console.error('Error setting client order ID:', err);
                    }
                }
            }
        }

        // Load current client order ID from server on page load only
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

        // Load sequence number once on page load
        loadInitialSequenceNumber();

        // Load client order ID once on page load
        loadInitialClientOrderId();

        // Sync to server when user changes the value
        const seqInput = form.querySelector('[name="sequenceNumber"]');
        if (seqInput) {
            seqInput.addEventListener('change', syncSequenceNumberToServer);
        }

        // Sync to server when user changes the client order ID
        const clientOrderIdInput = form.querySelector('[name="clientOrderId"]');
        if (clientOrderIdInput) {
            clientOrderIdInput.addEventListener('change', syncClientOrderIdToServer);
        }

        // Handle form submission
        form.addEventListener('submit', async (e) => {
            e.preventDefault();

            const formData = new FormData(form);
            // Convert FormData to URLSearchParams for proper URL encoding
            const urlEncoded = new URLSearchParams(formData);

            try {
                const response = await fetch(window.location.pathname, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: urlEncoded
                });

                const data = await response.json();

                if (data.status === 'success') {
                    statusMessage.innerHTML = '<span class="status-message status-success">' + data.message + '</span>';
                    // Increment global sequence number on server
                    await incrementSequenceNumber();
                    // Increment client order ID for order new submissions
                    if (pageType === 'ordernew') {
                        await incrementClientOrderId();
                    }
                } else {
                    statusMessage.innerHTML = '<span class="status-message status-error">Error: ' + data.message + '</span>';
                }

                // Clear status after 3 seconds
                setTimeout(() => {
                    statusMessage.innerHTML = '';
                }, 3000);

            } catch (err) {
                statusMessage.innerHTML = '<span class="status-message status-error">Network error: ' + err.message + '</span>';
            }
        });

        // Fetch and update both requests and responses every 500ms
        setInterval(async () => {
            try {
                // Fetch requests
                const reqResponse = await fetch('/' + pageType + '-requests');
                const requests = await reqResponse.json();

                if (requests && requests.length > 0) {
                    requestList.innerHTML = requests.slice().reverse().map(r =>
                        '<div class="message-box"><pre>' + r + '</pre></div>'
                    ).join('');
                } else {
                    requestList.innerHTML = '<p>No requests yet.</p>';
                }

                // Fetch responses
                const respResponse = await fetch('/' + pageType + '-responses');
                const responses = await respResponse.json();

                if (responses && responses.length > 0) {
                    responseList.innerHTML = responses.slice().reverse().map(r =>
                        '<div class="message-box"><pre>' + r + '</pre></div>'
                    ).join('');
                } else {
                    responseList.innerHTML = '<p>No responses yet.</p>';
                }
            } catch (err) {
                console.error('Error fetching data:', err);
            }
        }, 500);
    </script>
</body>
</html>
`

	data := struct {
		Type      string
		Title     string
		Responses []string
	}{
		Type:      pageType,
		Title:     getTitleForType(pageType),
		Responses: responses,
	}

	t, err := template.New("page").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, data)
}

func getTitleForType(pageType string) string {
	switch pageType {
	case "heartbeat":
		return "Heartbeat"
	case "ordernew":
		return "New Order"
	case "ordercancel":
		return "Cancel Order"
	default:
		return "Unknown"
	}
}

func main() {
	serverAddr := "localhost:50051" // Change this to your gRPC server address

	client, err := NewWebClient(serverAddr)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/entrypools", http.StatusSeeOther)
	})
	http.HandleFunc("/entrypools", client.handleEntryPools)
	http.HandleFunc("/entrypools-data", client.handleEntryPoolsData)
	http.HandleFunc("/heartbeat", client.handleHeartbeat)
	http.HandleFunc("/heartbeat-requests", client.handleHeartbeatRequests)
	http.HandleFunc("/heartbeat-responses", client.handleHeartbeatResponses)
	http.HandleFunc("/ordernew", client.handleOrderNew)
	http.HandleFunc("/ordernew-requests", client.handleOrderNewRequests)
	http.HandleFunc("/ordernew-responses", client.handleOrderNewResponses)
	http.HandleFunc("/ordercancel", client.handleOrderCancel)
	http.HandleFunc("/ordercancel-requests", client.handleOrderCancelRequests)
	http.HandleFunc("/ordercancel-responses", client.handleOrderCancelResponses)
	http.HandleFunc("/sequence-number", client.handleSequenceNumber)
	http.HandleFunc("/client-order-id", client.handleClientOrderId)

	log.Println("Starting web server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
