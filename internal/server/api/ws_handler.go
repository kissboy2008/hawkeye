package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for personal use
	},
}

// client wraps a WebSocket connection with a buffered send channel
// to avoid concurrent writes on the same conn.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains a set of active WebSocket connections and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]bool)}
}

func (h *Hub) Broadcast(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- message:
		default:
			// Client send buffer full, drop message to avoid blocking
			log.Printf("[ws] client send buffer full, dropping message")
		}
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	c.conn.Close()
}

// writePump pumps messages from the send channel to the WebSocket connection.
// Only one goroutine writes to the conn, eliminating concurrent write issues.
func (c *client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func handleWebSocket(hub *Hub, db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Authenticate via cookie (auto-sent by browser on same-origin WS)
		token, _ := c.Cookie("auth_token")
		if token == "" {
			token = c.Query("token") // fallback for backward compat
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		if _, err := db.ValidateToken(token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}

		cl := &client{
			conn: conn,
			send: make(chan []byte, 256),
		}

		hub.register(cl)

		// Start write pump in a separate goroutine
		go cl.writePump()

		// Read loop (keep alive, detect disconnect)
		defer hub.unregister(cl)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}
}

// BroadcastMetrics sends metrics data to all connected WebSocket clients.
func BroadcastMetrics(hub *Hub, agentID int64, metrics interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type":     "metrics",
		"agent_id": agentID,
		"data":     metrics,
	})
	if err != nil {
		return
	}
	hub.Broadcast(data)
}

// BroadcastProbeResult sends probe result to all connected WebSocket clients.
func BroadcastProbeResult(hub *Hub, result interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type": "probe_result",
		"data": result,
	})
	if err != nil {
		return
	}
	hub.Broadcast(data)
}

// BroadcastAlert sends alert event to all connected WebSocket clients.
func BroadcastAlert(hub *Hub, event interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type": "alert",
		"data": event,
	})
	if err != nil {
		return
	}
	hub.Broadcast(data)
}
