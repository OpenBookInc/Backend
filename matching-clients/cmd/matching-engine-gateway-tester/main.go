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
	gwpb "matching-clients/src/gen/gateway"
	tc "matching-clients/src/tester_common"
	"matching-clients/src/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:embed index.html
var indexHTML string

type WebClient struct {
	conn *grpc.ClientConn
	mu   sync.RWMutex

	gwClient   gwpb.GatewayServerServiceClient
	gwStream   gwpb.GatewayServerService_CreateTradeStreamClient
	gwSendChan chan *gwpb.BackendMessage

	// Unified request/response storage
	requests  []string
	responses []string

	tradeStreamActive bool
	streamGeneration  uint64 // Incremented on each reconnect to invalidate old goroutines

	// Global client order ID counter
	globalClientOrderId uint64

	// Pool state tracking
	poolTracker   *tc.PoolTracker
	pendingOrders map[uint64]*tc.PendingOrderInfo // map[clientOrderId]PendingOrderInfo

	// Connection info
	serverHost     string
	gatewayPort    string
	gatewayUserIDs []string
	gatewayLegIDs  []string
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
		gatewayPort:         cfg.GatewayPort,
		gatewayUserIDs:      cfg.GatewayUserIDs,
		gatewayLegIDs:       cfg.GatewayLegIDs,
	}

	// Try to connect to the gateway, but don't fail if unreachable
	if err := wc.Connect(); err != nil {
		log.Printf("Warning: Could not connect to gateway: %v", err)
		log.Printf("Web server will start in disconnected state")
	}

	return wc
}

// processGatewayMessage processes gateway responses and updates pool state.
// Pool state is driven entirely by OrderPoolSnapshot messages from the gateway;
// ack/match/elimination events are not used for pool tracking since the gateway
// sends an updated snapshot after every state change.
// NOTE: This method assumes wc.mu is already locked by the caller
func (wc *WebClient) processGatewayMessage(resp *gwpb.GatewayMessage) {
	switch event := resp.Event.(type) {
	case *gwpb.GatewayMessage_OrderPoolSnapshot:
		snapshot := event.OrderPoolSnapshot
		if snapshot != nil {
			var legSecurityIDs []utils.UUID
			for _, legID := range snapshot.GetLegSecurityIds() {
				legSecurityIDs = append(legSecurityIDs, utils.UUIDFromUint64(legID.GetUpper(), legID.GetLower()))
			}

			var lineupBooks []tc.LineupBookSnapshot
			for _, book := range snapshot.GetLineupBooks() {
				lb := tc.LineupBookSnapshot{
					IsOver: book.GetIsOver(),
				}
				for _, level := range book.GetLevels() {
					ls := tc.LevelSnapshot{Portion: level.GetPortion()}
					for _, order := range level.GetOrders() {
						ls.Orders = append(ls.Orders, tc.OrderSnapshot{
							QuantityRemaining: order.GetQuantityRemaining(),
						})
					}
					lb.Levels = append(lb.Levels, ls)
				}
				lineupBooks = append(lineupBooks, lb)
			}

			wc.poolTracker.ReplacePoolFromSnapshot(legSecurityIDs, lineupBooks)
		}
	}
}

func (wc *WebClient) Close() {
	if wc.gwSendChan != nil {
		close(wc.gwSendChan)
	}
	if wc.gwStream != nil {
		wc.gwStream.CloseSend()
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

// handleHealthCheck returns TCP reachability of the gateway service
func (wc *WebClient) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	host := wc.serverHost
	gatewayPort := wc.gatewayPort
	wc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"gateway": map[string]interface{}{
			"reachable": checkServiceReachable(host, gatewayPort),
			"port":      gatewayPort,
		},
	})
}

// Connect establishes a connection to the gateway.
func (wc *WebClient) Connect() error {
	wc.mu.Lock()

	wc.streamGeneration++
	currentGen := wc.streamGeneration
	wc.tradeStreamActive = false

	// Capture old resources
	oldGwSendChan := wc.gwSendChan
	oldGwStream := wc.gwStream
	oldConn := wc.conn

	// Clear references
	wc.gwSendChan = nil
	wc.gwStream = nil
	wc.gwClient = nil
	wc.conn = nil

	// Clear state
	wc.requests = make([]string, 0)
	wc.responses = make([]string, 0)
	wc.poolTracker = tc.NewPoolTracker()
	wc.pendingOrders = make(map[uint64]*tc.PendingOrderInfo)
	wc.globalClientOrderId = 1000

	serverHost := wc.serverHost
	gatewayPort := wc.gatewayPort

	wc.mu.Unlock()

	// Close old resources outside the lock
	if oldGwSendChan != nil {
		close(oldGwSendChan)
	}
	if oldGwStream != nil {
		oldGwStream.CloseSend()
	}
	if oldConn != nil {
		oldConn.Close()
	}

	time.Sleep(100 * time.Millisecond)

	serverAddr := fmt.Sprintf("%s:%s", serverHost, gatewayPort)
	if !checkServiceReachable(serverHost, gatewayPort) {
		log.Printf("Gateway at %s is not reachable", serverAddr)
		return fmt.Errorf("gateway at %s is not reachable", serverAddr)
	}

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", serverAddr, err)
		return fmt.Errorf("failed to connect to %s: %v", serverAddr, err)
	}

	client := gwpb.NewGatewayServerServiceClient(conn)
	stream, err := client.CreateTradeStream(context.Background())
	if err != nil {
		conn.Close()
		log.Printf("Failed to create trade stream on %s: %v", serverAddr, err)
		return fmt.Errorf("failed to create trade stream: %v", err)
	}

	wc.mu.Lock()
	wc.conn = conn
	wc.gwClient = client
	wc.gwStream = stream
	wc.gwSendChan = make(chan *gwpb.BackendMessage, 10)
	wc.tradeStreamActive = true
	sendChan := wc.gwSendChan
	wc.mu.Unlock()

	// Send OrderPoolSyncRequest to get current pool state
	syncMsg := &gwpb.BackendMessage{
		Msg: &gwpb.BackendMessage_OrderPoolSyncRequest{
			OrderPoolSyncRequest: &gwpb.OrderPoolSyncRequest{},
		},
	}
	if err := stream.Send(syncMsg); err != nil {
		log.Printf("Failed to send OrderPoolSyncRequest: %v", err)
	} else {
		log.Println("Sent OrderPoolSyncRequest to gateway")
	}

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
				log.Println("Gateway stream closed by server")
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
				wc.processGatewayMessage(resp)
			}
			wc.mu.Unlock()
		}
	}()

	log.Printf("Connected to gateway at %s", serverAddr)
	return nil
}

// handleReconnect handles POST requests to reconnect to the gateway
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
	wc.sendGatewayOrder(w, r, messageType)
}

// sendGatewayOrder builds and sends a gateway proto BackendMessage
func (wc *WebClient) sendGatewayOrder(w http.ResponseWriter, r *http.Request, messageType string) {
	marshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}

	var backendMsg *gwpb.BackendMessage

	switch messageType {
	case "NewOrder":
		clientOrderId, _ := strconv.ParseUint(r.FormValue("clientOrderId"), 10, 64)

		var userIDProto *common.UUID
		if v := r.FormValue("userId"); v != "" {
			if parsed, err := utils.ParseUUID(v); err == nil {
				userIDProto = &common.UUID{Upper: parsed.Upper(), Lower: parsed.Lower()}
			}
		}

		trackerLegs, legUUIDs := tc.ParseLegsFromForm(r.FormValue("legSecurityIds"), r.FormValue("isOvers"))

		var gwLegs []*gwpb.NewOrder_Body_Leg
		for i, uuid := range legUUIDs {
			gwLegs = append(gwLegs, &gwpb.NewOrder_Body_Leg{
				LegSecurityId: uuid,
				IsOver:        trackerLegs[i].IsOver,
			})
		}

		orderType := common.OrderType_LIMIT
		if r.FormValue("orderType") == "MARKET" {
			orderType = common.OrderType_MARKET
		}
		portion, _ := strconv.ParseUint(r.FormValue("portion"), 10, 64)
		quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 64)

		backendMsg = &gwpb.BackendMessage{
			Msg: &gwpb.BackendMessage_NewOrder{
				NewOrder: &gwpb.NewOrder{
					Body: &gwpb.NewOrder_Body{
						UserId:        userIDProto,
						ClientOrderId: clientOrderId,
						Legs:          gwLegs,
						OrderType:     orderType,
						Portion:       portion,
						Quantity:      quantity,
					},
				},
			},
		}

		wc.mu.Lock()
		wc.pendingOrders[clientOrderId] = &tc.PendingOrderInfo{
			Legs:     trackerLegs,
			Portion:  portion,
			Quantity: quantity,
		}
		wc.mu.Unlock()

	case "CancelOrder":
		orderIdStr := r.FormValue("orderId")
		orderUUID, err := utils.ParseUUID(orderIdStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": "error", "message": "Invalid UUID for order ID"}`))
			return
		}

		backendMsg = &gwpb.BackendMessage{
			Msg: &gwpb.BackendMessage_CancelOrder{
				CancelOrder: &gwpb.CancelOrder{
					Body: &gwpb.CancelOrder_Body{
						OrderId: &common.UUID{Upper: orderUUID.Upper(), Lower: orderUUID.Lower()},
					},
				},
			},
		}

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status": "error", "message": "Invalid message type"}`))
		return
	}

	reqJSON, _ := marshaler.Marshal(backendMsg)
	wc.mu.Lock()
	wc.requests = append(wc.requests, string(reqJSON))
	wc.mu.Unlock()

	select {
	case wc.gwSendChan <- backendMsg:
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

// TemplateData holds data passed to the HTML template
type TemplateData struct {
	GatewayUserIDs []string
	GatewayLegIDs  []string
}

// Trade page handler - displays the gateway tester interface
func (wc *WebClient) handleTrade(w http.ResponseWriter, r *http.Request) {
	wc.mu.RLock()
	data := TemplateData{
		GatewayUserIDs: wc.gatewayUserIDs,
		GatewayLegIDs:  wc.gatewayLegIDs,
	}
	wc.mu.RUnlock()
	renderMainPage(w, data)
}

func renderMainPage(w http.ResponseWriter, data TemplateData) {
	t, err := template.New("main").Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, data)
}


func main() {
	cfg, err := tc.LoadConfig()
	if err != nil {
		tc.Fatal("Failed to load configuration: %v\nPlease ensure .env file exists with required fields (SERVER_HOST, SERVER_PORT)", err)
	}

	log.Printf("Starting Gateway Tester")

	client := NewWebClient(cfg)
	defer client.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/trade", http.StatusSeeOther)
	})
	http.HandleFunc("/trade", client.handleTrade)
	http.HandleFunc("/send", client.handleSendOrder)
	http.HandleFunc("/requests", client.handleRequests)
	http.HandleFunc("/responses", client.handleResponses)
	http.HandleFunc("/client-order-id", client.handleClientOrderId)
	http.HandleFunc("/entrypools-data", client.handleEntryPoolsData)
	http.HandleFunc("/reconnect", client.handleReconnect)
	http.HandleFunc("/health-check", client.handleHealthCheck)

	webAddr := fmt.Sprintf(":%s", cfg.WebPort)
	log.Printf("Starting web server on %s", webAddr)
	log.Fatal(http.ListenAndServe(webAddr, nil))
}
