package sse

import (
	"backend/common"
	"backend/triggers"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lib/pq"
)

type SSEManager struct {
	clients        map[chan string]struct{}
	clientMux      sync.RWMutex
	triggerManager *triggers.TriggerManager
	priceUpdates   chan PriceUpdate
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
	}

	go sse.handlePriceUpdates()
	return sse
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

// Broadcast price updates
func (sse *SSEManager) BroadcastPrice(update PriceUpdate) {
	// log.Printf("\nSSE:BroadcastPrice\n Product: %s\n Price: %f\n Time: %d\n", update.ProductID, update.Price, update.Timestamp)
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
	message, err := json.Marshal(map[string]interface{}{
		"event": "trigger",
		"data":  trigger,
	})
	if err != nil {
		log.Printf("Error marshaling trigger: %v", err)
		return
	}
	sse.broadcastMessage(string(message))
}

func (sse *SSEManager) ListenForDBChanges(dsn string, channel string) {
	log.Println("SSE Listen For DB Changes")
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

		// log.Printf("Received notification: %+v\n", notification)

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

		switch payload.Table {
		case "triggers":
			var trigger common.Trigger
			if err := json.Unmarshal(payload.Data, &trigger); err != nil {
				log.Printf("Error parsing trigger data: %v", err)
				continue
			}
			sse.triggerManager.UpdateTriggers([]common.Trigger{trigger})
			sse.BroadcastTrigger(trigger)

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
	sse.clientMux.RLock()
	defer sse.clientMux.Unlock()
	sse.clients[client] = struct{}{}
}

func (sse *SSEManager) RemoveClient(client chan string) {
	sse.clientMux.Lock()
	defer sse.clientMux.Unlock()
	delete(sse.clients, client)
	close(client)
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
	log.Println("New SSE client connected")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan string)
	sse.AddClient(client)
	defer sse.RemoveClient(client)

	for message := range client {
		fmt.Fprintf(w, "data: %s\n\n", message)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
