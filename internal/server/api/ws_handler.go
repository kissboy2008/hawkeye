package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	wsPingInterval = 30 * time.Second // 服务端主动 ping，探测半开连接
	wsPongWait     = 70 * time.Second // 超过两个 ping 周期没 pong 就断开
	wsWriteWait    = 10 * time.Second // 单次写超时
)

var upgrader = websocket.Upgrader{
	// 之前恒 true：任意网站的页面都能连上本机 WS（拿着用户的 cookie/query token 也一样）。
	// 改成同源校验：Origin 为空（curl / 原生客户端）放行，否则必须与 Host 一致。
	CheckOrigin: originAllowed,
}

// originAllowed 判断 WS 握手的 Origin 是否可信。
// 反向代理（lucky/nginx）转发时可能改写 Host，原始域名只留在 X-Forwarded-Host 里，
// 所以两个都要比对，否则带代理的部署会被自己的校验挡掉。
// 浏览器无法伪造 X-Forwarded-Host，非浏览器客户端本来也不需要绕过 CSRF 防护。
func originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == "null" { // file:// / 沙箱 iframe，一律拒绝
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	candidates := []string{r.Host}
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		candidates = append(candidates, strings.Split(xfh, ",")...)
	}
	for _, c := range candidates {
		if strings.EqualFold(strings.TrimSpace(c), u.Host) {
			return true
		}
	}
	log.Printf("[ws] origin rejected: origin=%s host=%s xfh=%s", origin, r.Host, r.Header.Get("X-Forwarded-Host"))
	return false
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
	ticker := time.NewTicker(wsPingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func handleWebSocket(hub *Hub, db *storage.DB, authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authDisabled {
			// 优先用 cookie（浏览器）；query 参数只为兼容老客户端保留，
			// 注意 query 里的 token 可能被上游反代/浏览器历史记录留存。
			token, _ := c.Cookie("auth_token")
			if token == "" {
				token = c.Query("token")
			}
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
				return
			}
			masked := token
			if len(token) > 8 {
				masked = token[:4] + "..." + token[len(token)-4:]
			}
			if _, err := db.ValidateToken(token); err != nil {
				log.Printf("[ws] auth rejected: %v (token=%s)", err, masked)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}
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
		// 读超时 + pong 续期：没有这两个，NAT/手机上滑掉客户端后服务端永远留着半开连接，
		// 广播时不断往死连接写（以前要靠 send buffer 满才打日志）。
		defer hub.unregister(cl)
		conn.SetReadLimit(4096)
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(wsPongWait))
		})
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
