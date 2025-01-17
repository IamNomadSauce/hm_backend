package sse

import (
	"backend/common"
	"backend/triggers"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

type SSEManager struct {
	clients         map[chan string]struct{}
	clientMux       sync.RWMutex
	triggerManager  *triggers.TriggerManager
	priceUpdates    chan PriceUpdate
	activeConns     map[string]bool
	candleUpdates   chan common.Candle
	selectedProduct string
	selectedTable   string
}

type PriceUpdate struct {
	ProductID string  `json:"product_id"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

func NewSSEManager(triggerManager *triggers.TriggerManager) *SSEManager {
	log.Println("New SSE Manager")
	sse := &SSEManager{
		clients:        make(map[chan string]struct{}),
		triggerManager: triggerManager,
		priceUpdates:   make(chan PriceUpdate, 100),
		activeConns:    make(map[string]bool),
		candleUpdates:  make(chan common.Candle, 100),
	}

	go sse.handlePriceUpdates()
	go sse.handleCandleUpdates()
	go sse.handleActiveConns()
	return sse
}

func (sse *SSEManager) UpdateSelectedProduct(tableName, productID string) {
	sse.clientMux.Lock()
	defer sse.clientMux.Unlock()

	sse.selectedTable = tableName
	sse.selectedProduct = productID

	log.Printf("SSE Manager updated to watch table: %s for product: %s", tableName, productID)
}

func (sse *SSEManager) handlePriceUpdates() {
	// log.Println("SSE:Handle Price Updates", sse.priceUpdates)
	for update := range sse.priceUpdates {
		// log.Println("Price_Update:", update)
		// Process triggers
		if triggeredTriggers := sse.triggerManager.ProcessPriceUpdate(update.ProductID, update.Price); len(triggeredTriggers) > 0 {
			for _, trigger := range triggeredTriggers {
				sse.BroadcastTrigger(trigger)
			}
		}

		// Broadcast price update
		sse.BroadcastPrice(update)
	}
}

func (sse *SSEManager) handleCandleUpdates() {
	log.Println("SSE.HandleCandleUpdates")
	for candle := range sse.candleUpdates {
		log.Printf("Handle Candle Updates %+v", candle)
		sse.BroadcastCandle(candle)
	}
}

func (sse *SSEManager) handleActiveConns() {
	log.Println("SSE.HandleActiveConns")
	for connection := range sse.activeConns {
		log.Printf("Connection: %+v", connection)
	}
}

func (sse *SSEManager) BroadcastPrice(update PriceUpdate) {
	message, err := json.Marshal(map[string]interface{}{
		"event": "price",
		"data":  update,
	})
	if err != nil {
		log.Printf("Error marshaling price update: %v", err)
		return
	}
	sse.broadcastMessage(string(message))
}

func (sse *SSEManager) BroadcastCandle(candle common.Candle) {
	message, err := json.Marshal(map[string]interface{}{
		"event": "candle",
		"data":  candle,
	})
	if err != nil {
		log.Printf("Error marshaling candle update: %v", err)
		return
	}
	sse.broadcastMessage(string(message))
}

// Base broadcast method for string messages
func (sse *SSEManager) broadcastMessage(message string) {
	sse.clientMux.RLock()
	defer sse.clientMux.RUnlock()
	for client := range sse.clients {
		select {
		case client <- message:
		default:
			log.Println("Client channel is full, dropping message")
		}
	}
}

// Broadcast trigger updates
func (sse *SSEManager) BroadcastTrigger(trigger common.Trigger) {
	// log.Println("SSE:BroadcastTrigger", trigger)
	message := fmt.Sprintf("event: trigger\ndata: %s\n\n", toJSON(trigger))
	sse.broadcastMessage(message)
}

func (sse *SSEManager) ListenForDBChanges(dsn string, channel string, selectedProduct string) {
	log.Println("SSE.ListenForDBChanges Initialized")
	listener := pq.NewListener(dsn, 10*time.Second, time.Minute,
		func(ev pq.ListenerEventType, err error) {
			if err != nil {
				log.Printf("Listener error: %v", err)
			}
		})

	if err := listener.Listen(channel); err != nil {
		log.Fatalf("Listen error: %v", err)
	}

	log.Printf("Listening on channel: %s", channel)

	// Add ping routine to keep connection alive
	go func() {
		for {
			time.Sleep(30 * time.Second)
			if err := listener.Ping(); err != nil {
				log.Printf("Error pinging listener: %v", err)
			}
		}
	}()

	for notification := range listener.Notify {
		if notification == nil {
			continue
		}

		var payload struct {
			Table     string          `json:"table"`
			Operation string          `json:"operation"`
			Data      json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal([]byte(notification.Extra), &payload); err != nil {
			log.Printf("Error parsing notification: %v", err)
			continue
		}

		// ------

		table := payload.Table
		// log.Printf("Received notification: %+v\n", notification)
		// log.Printf("Table: %s\nSelectedProduct: %s", table, sse.selectedTable)
		// log.Printf("Table: %s", table)

		var product_table string

		if strings.Contains(table, sse.selectedTable) {
			product_table = table
		}

		switch table {
		case "triggers":
			var trigger common.Trigger
			if err := json.Unmarshal(payload.Data, &trigger); err != nil {
				log.Printf("Error parsing trigger data: %v", err)
				continue
			}
			sse.triggerManager.UpdateTriggers([]common.Trigger{trigger})
			sse.BroadcastTrigger(trigger)

		case product_table:
			var rawCandle struct {
				Timestamp int64   `json:"timestamp"`
				Open      float64 `json:"open"`
				High      float64 `json:"high"`
				Low       float64 `json:"low"`
				Close     float64 `json:"close"`
				Volume    float64 `json:"volume"`
			}

			if err := json.Unmarshal(payload.Data, &rawCandle); err != nil {
				log.Printf("Error parsing raw candle data: %v", err)
				continue
			}

			candle := common.Candle{
				ProductID: sse.selectedProduct,
				Timestamp: rawCandle.Timestamp,
				Open:      rawCandle.Open,
				High:      rawCandle.High,
				Low:       rawCandle.Low,
				Close:     rawCandle.Close,
				Volume:    rawCandle.Volume, // Fixed: was using Open instead of Volume
			}

			// log.Printf("Candle update for %s: %v", table, candle)
			sse.BroadcastCandle(candle)
		}

		if payload.Table == "triggers" {
			log.Println("Trigger:", payload.Table)
			log.Printf("Parsed payload - Table: %s, Operation: %s", payload.Table, payload.Operation)
		}
	}
}

func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshalling to JSON: %v", err)
		return "{}"
	}
	return string(data)
}

func (sse *SSEManager) AddClient(client chan string) {
	sse.clientMux.Lock() // Change from RLock to Lock
	defer sse.clientMux.Unlock()
	sse.clients[client] = struct{}{}
}

func (sse *SSEManager) RemoveClient(client chan string) {
	log.Println("SSE:Remove Client")
	sse.clientMux.Lock()
	defer sse.clientMux.Unlock()
	if _, exists := sse.clients[client]; exists {
		delete(sse.clients, client)
		close(client)
	}
}

// func (sse *SSEManager) Broadcast(message string) {
// 	sse.clientMux.RLock()
// 	defer sse.clientMux.RUnlock()
// 	for client := range sse.clients {
// 		select {
// 		case client <- message:
// 		default:
// 			log.Println("Client channel is full, dropping message")
// 		}
// 	}
// }

func (sse *SSEManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("New SSE connection attempt from %s", r.RemoteAddr)
	// log.Printf("Request headers: %+v", r.Header)

	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080") // Match your frontend origin
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Handle preflight
	if r.Method == "OPTIONS" {
		log.Printf("Handling OPTIONS preflight request from %s", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush headers immediately
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
		// log.Printf("Headers flushed for client %s", r.RemoteAddr)
	} else {
		log.Printf("Warning: ResponseWriter doesn't support Flushing for client %s", r.RemoteAddr)
	}

	if err := sse.triggerManager.ReloadTriggers(); err != nil {
		log.Printf("Error reloading triggers: %v", err)
	}

	client := make(chan string, 1000)
	sse.AddClient(client)
	log.Printf("Client %s added to SSE manager", r.RemoteAddr)

	// Monitor connection state
	notify := r.Context().Done()
	go func() {
		<-notify
		log.Printf("Client %s connection closed", r.RemoteAddr)
		sse.RemoveClient(client)
	}()

	for message := range client {
		// log.Printf("Sending message to client %s: %s", r.RemoteAddr, message)
		fmt.Fprintf(w, "data: %s\n\n", message)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
