package api

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	nodeID string
}

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex

	latestSnapshot []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			// Send initial state
			h.mu.RLock()
			if h.latestSnapshot != nil {
				select {
				case client.send <- h.latestSnapshot:
				default:
				}
			}
			h.mu.RUnlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			h.latestSnapshot = message
			h.mu.Unlock()

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// client buffer full, drop
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[hub] json marshal error: %v", err)
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastMessage(msgType string, data interface{}) {
	msg := map[string]interface{}{
		"type": msgType,
		"data": data,
	}
	h.BroadcastJSON(msg)
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
