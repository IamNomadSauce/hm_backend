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
	listener := pq.NewListener(
		dsn,            // Pass the DSN directly
		10*time.Second, // MinReconnectInterval
		time.Minute,    // MaxReconnectInterval
		func(ev pq.ListenerEventType, err error) {
			if err != nil {
				log.Printf("Postgres listener error: %v", err)
			}
		},
	)

	err := listener.Listen(channel)
	if err != nil {
		log.Fatalf("Error listening to channel %s: %v", channel, err)
	}

	log.Printf("Listening for database changes on channel: %s", channel)

	for {
		select {
		case notification := <-listener.Notify:
			if notification != nil {
				var payload map[string]interface{}
				err := json.Unmarshal([]byte(notification.Extra), &payload)
				if err != nil {
					log.Printf("Error unmarshalling notification payload: %v", err)
					continue
				}

				message := fmt.Sprintf("DB Change - Channel: %s, Payload: %+v", channel, payload)
				sse.Broadcast(message)
			}
		case <-time.After(90 * time.Second):
			log.Println("No notifications received in 90 seconds, checking connection...")
			go func() {
				err := listener.Ping()
				if err != nil {
					log.Printf("Listener ping error: %v", err)
				}
			}()
		}
	}
}
