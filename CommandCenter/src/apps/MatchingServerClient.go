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
)

type WebClient struct {
	conn   *grpc.ClientConn
	client pb.MatchingServerServiceClient
	mu     sync.RWMutex
	
	// Response storage
	heartbeatResponses     []string
	poolDefResponses       []string
	orderNewResponses      []string
	orderCancelResponses   []string
}

func NewWebClient(serverAddr string) (*WebClient, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	return &WebClient{
		conn:   conn,
		client: pb.NewMatchingServerServiceClient(conn),
		heartbeatResponses:   make([]string, 0),
		poolDefResponses:     make([]string, 0),
		orderNewResponses:    make([]string, 0),
		orderCancelResponses: make([]string, 0),
	}, nil
}

func (wc *WebClient) Close() {
	if wc.conn != nil {
		wc.conn.Close()
	}
}

// Heartbeat handler
func (wc *WebClient) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		stream, err := wc.client.OnHeartbeat(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create stream: %v", err), http.StatusInternalServerError)
			return
		}

		// Send heartbeat
		heartbeat := &pb.Heartbeat{
			MessageBase: &pb.MessageBase{
				VersionMajor: 1,
				VersionMinor: 0,
				MessageType:  pb.MessageType_HEARTBEAT,
			},
		}

		if err := stream.Send(heartbeat); err != nil {
			http.Error(w, fmt.Sprintf("Failed to send: %v", err), http.StatusInternalServerError)
			return
		}

		// Receive response
		resp, err := stream.Recv()
		if err != nil && err != io.EOF {
			http.Error(w, fmt.Sprintf("Failed to receive: %v", err), http.StatusInternalServerError)
			return
		}

		stream.CloseSend()

		respJSON, _ := json.MarshalIndent(resp, "", "  ")
		wc.mu.Lock()
		wc.heartbeatResponses = append(wc.heartbeatResponses, string(respJSON))
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	renderPage(w, "heartbeat", wc.heartbeatResponses)
}

// Pool Definition Request handler
func (wc *WebClient) handlePoolDefinition(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		legIdsStr := r.FormValue("legSecurityIds")
		
		var legIds []uint64
		if legIdsStr != "" {
			for _, idStr := range splitAndTrim(legIdsStr, ",") {
				id, _ := strconv.ParseUint(idStr, 10, 64)
				legIds = append(legIds, id)
			}
		}

		req := &pb.PoolDefinitionRequest{
			MessageBase: &pb.MessageBase{
				VersionMajor: 1,
				VersionMinor: 0,
				MessageType:  pb.MessageType_POOL_DEFINITION_REQUEST,
			},
			SequencedMessageBase: &pb.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			LegSecurityIds: legIds,
		}

		resp, err := wc.client.OnPoolDefinitionRequest(context.Background(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to send request: %v", err), http.StatusInternalServerError)
			return
		}

		respJSON, _ := json.MarshalIndent(resp, "", "  ")
		wc.mu.Lock()
		wc.poolDefResponses = append(wc.poolDefResponses, string(respJSON))
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	renderPage(w, "pooldef", wc.poolDefResponses)
}

// Order New handler
func (wc *WebClient) handleOrderNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		lineupId, _ := strconv.ParseUint(r.FormValue("lineupId"), 10, 64)
		orderType := pb.OrderType_LIMIT
		if r.FormValue("orderType") == "MARKET" {
			orderType = pb.OrderType_MARKET
		}
		price, _ := strconv.ParseUint(r.FormValue("price"), 10, 64)
		quantity, _ := strconv.ParseUint(r.FormValue("quantity"), 10, 64)
		selfMatchId, _ := strconv.ParseUint(r.FormValue("selfMatchId"), 10, 64)

		stream, err := wc.client.OnOrderNew(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create stream: %v", err), http.StatusInternalServerError)
			return
		}

		order := &pb.OrderNew{
			MessageBase: &pb.MessageBase{
				VersionMajor: 1,
				VersionMinor: 0,
				MessageType:  pb.MessageType_ORDER_NEW,
			},
			SequencedMessageBase: &pb.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			LineupId:    lineupId,
			OrderType:   orderType,
			Price:       price,
			Quantity:    quantity,
			SelfMatchId: selfMatchId,
		}

		if err := stream.Send(order); err != nil {
			http.Error(w, fmt.Sprintf("Failed to send: %v", err), http.StatusInternalServerError)
			return
		}

		var responses []interface{}
		go func() {
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				responses = append(responses, resp)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		stream.CloseSend()

		respJSON, _ := json.MarshalIndent(responses, "", "  ")
		wc.mu.Lock()
		wc.orderNewResponses = append(wc.orderNewResponses, string(respJSON))
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	renderPage(w, "ordernew", wc.orderNewResponses)
}

// Order Cancel handler
func (wc *WebClient) handleOrderCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		seqNum, _ := strconv.ParseUint(r.FormValue("sequenceNumber"), 10, 64)
		orderId, _ := strconv.ParseUint(r.FormValue("orderId"), 10, 64)

		stream, err := wc.client.OnOrderCancel(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create stream: %v", err), http.StatusInternalServerError)
			return
		}

		cancel := &pb.OrderCancel{
			MessageBase: &pb.MessageBase{
				VersionMajor: 1,
				VersionMinor: 0,
				MessageType:  pb.MessageType_ORDER_CANCEL,
			},
			SequencedMessageBase: &pb.SequencedMessageBase{
				SequenceNumber: seqNum,
			},
			OrderId: orderId,
		}

		if err := stream.Send(cancel); err != nil {
			http.Error(w, fmt.Sprintf("Failed to send: %v", err), http.StatusInternalServerError)
			return
		}

		var responses []interface{}
		go func() {
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				responses = append(responses, resp)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		stream.CloseSend()

		respJSON, _ := json.MarshalIndent(responses, "", "  ")
		wc.mu.Lock()
		wc.orderCancelResponses = append(wc.orderCancelResponses, string(respJSON))
		wc.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	renderPage(w, "ordercancel", wc.orderCancelResponses)
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

func renderPage(w http.ResponseWriter, pageType string, responses []string) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Matching Service Client - {{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        .nav { margin-bottom: 20px; }
        .nav a { margin-right: 15px; padding: 5px 10px; text-decoration: none; background: #007bff; color: white; border-radius: 3px; }
        .nav a:hover { background: #0056b3; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, select { width: 300px; padding: 8px; border: 1px solid #ddd; border-radius: 3px; }
        button { padding: 10px 20px; background: #28a745; color: white; border: none; border-radius: 3px; cursor: pointer; }
        button:hover { background: #218838; }
        .responses { margin-top: 30px; }
        .response { background: #f8f9fa; padding: 15px; margin-bottom: 10px; border-radius: 3px; border: 1px solid #ddd; }
        pre { white-space: pre-wrap; word-wrap: break-word; }
    </style>
</head>
<body>
    <h1>Matching Service Client</h1>
    <div class="nav">
        <a href="/heartbeat">Heartbeat</a>
        <a href="/pooldef">Pool Definition</a>
        <a href="/ordernew">Order New</a>
        <a href="/ordercancel">Order Cancel</a>
    </div>
    
    {{if eq .Type "heartbeat"}}
    <h2>Heartbeat</h2>
    <form method="POST">
        <button type="submit">Send Heartbeat</button>
    </form>
    {{end}}
    
    {{if eq .Type "pooldef"}}
    <h2>Pool Definition Request</h2>
    <form method="POST">
        <div class="form-group">
            <label>Sequence Number:</label>
            <input type="number" name="sequenceNumber" value="1" required>
        </div>
        <div class="form-group">
            <label>Leg Security IDs (comma-separated):</label>
            <input type="text" name="legSecurityIds" placeholder="101,102,103" required>
        </div>
        <button type="submit">Send Request</button>
    </form>
    {{end}}
    
    {{if eq .Type "ordernew"}}
    <h2>New Order</h2>
    <form method="POST">
        <div class="form-group">
            <label>Sequence Number:</label>
            <input type="number" name="sequenceNumber" value="1" required>
        </div>
        <div class="form-group">
            <label>Lineup ID:</label>
            <input type="number" name="lineupId" value="1" required>
        </div>
        <div class="form-group">
            <label>Order Type:</label>
            <select name="orderType">
                <option value="LIMIT">LIMIT</option>
                <option value="MARKET">MARKET</option>
            </select>
        </div>
        <div class="form-group">
            <label>Price:</label>
            <input type="number" name="price" value="100" required>
        </div>
        <div class="form-group">
            <label>Quantity:</label>
            <input type="number" name="quantity" value="10" required>
        </div>
        <div class="form-group">
            <label>Self Match ID:</label>
            <input type="number" name="selfMatchId" value="0" required>
        </div>
        <button type="submit">Send Order</button>
    </form>
    {{end}}
    
    {{if eq .Type "ordercancel"}}
    <h2>Cancel Order</h2>
    <form method="POST">
        <div class="form-group">
            <label>Sequence Number:</label>
            <input type="number" name="sequenceNumber" value="1" required>
        </div>
        <div class="form-group">
            <label>Order ID:</label>
            <input type="number" name="orderId" required>
        </div>
        <button type="submit">Cancel Order</button>
    </form>
    {{end}}
    
    <div class="responses">
        <h3>Responses</h3>
        {{range .Responses}}
        <div class="response">
            <pre>{{.}}</pre>
        </div>
        {{else}}
        <p>No responses yet.</p>
        {{end}}
    </div>
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
	case "pooldef":
		return "Pool Definition"
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
		http.Redirect(w, r, "/heartbeat", http.StatusSeeOther)
	})
	http.HandleFunc("/heartbeat", client.handleHeartbeat)
	http.HandleFunc("/pooldef", client.handlePoolDefinition)
	http.HandleFunc("/ordernew", client.handleOrderNew)
	http.HandleFunc("/ordercancel", client.handleOrderCancel)

	log.Println("Starting web server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
