package main

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub mantem o conjunto de clientes WebSocket conectados e faz broadcast
// das mensagens recebidas no canal Broadcast.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool

	Broadcast chan []byte
	Register  chan *websocket.Conn
	Unreg     chan *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients:   map[*websocket.Conn]bool{},
		Broadcast: make(chan []byte, 256),
		Register:  make(chan *websocket.Conn),
		Unreg:     make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.Register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.Unreg:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.Close()
			}
			h.mu.Unlock()
		case msg := <-h.Broadcast:
			h.mu.RLock()
			for c := range h.clients {
				if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("[Hub] Erro escrevendo para cliente: %v. Removendo.", err)
					go func(cc *websocket.Conn) { h.Unreg <- cc }(c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// PushEvent serializa e enfileira um evento para broadcast.
func (h *Hub) PushEvent(eventType string, payload any) {
	msg := map[string]any{
		"type": eventType,
		"data": payload,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Hub] Falha ao serializar push: %v", err)
		return
	}
	select {
	case h.Broadcast <- b:
	default:
		log.Printf("[Hub] Broadcast cheio, descartando evento %s", eventType)
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
