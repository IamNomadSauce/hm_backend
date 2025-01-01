package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lib/pq"
)

type SSEManager struct {
	clients   map[chan string]struct{}
	clientMux sync.RWMutex
}

func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients: make(map[chan string]struct{}),
	}
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

func (sse *SSEManager) Broadcast(message string) {
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

func (sse *SSEManager) ListenForDBChanges(dsn string, channel string) {
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

		if payload.Table == "alerts" {
			log.Println("Alert:", payload.Table)
			log.Printf("Parsed payload - Table: %s, Operation: %s", payload.Table, payload.Operation)
		}

		message := fmt.Sprintf("Table %s %s: %s",
			payload.Table, payload.Operation, string(payload.Data))
		sse.Broadcast(message)
	}
}
