package api

import (
	"log"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// WebSocketMessage WebSocket 推送消息格式
type WebSocketMessage struct {
	Type      string      `json:"type"`      // "dashboard_update", "ping"
	Timestamp string      `json:"timestamp"` // RFC3339
	Data      interface{} `json:"data"`
}

// WebSocketHub 管理所有 WebSocket 客户端连接
type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan *WebSocketMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
}

// NewWebSocketHub 创建 WebSocket Hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan *WebSocketMessage, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动 Hub 的连接管理 goroutine
func (h *WebSocketHub) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	go h.run()
	log.Printf("[WebSocket] Hub started")
}

// Stop 停止 Hub，关闭所有客户端连接
func (h *WebSocketHub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	close(h.stopCh)

	for client := range h.clients {
		client.Close()
	}

	log.Printf("[WebSocket] Hub stopped")
}

// run 管理客户端连接的主循环
func (h *WebSocketHub) run() {
	for {
		select {
		case <-h.stopCh:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] Client connected, total: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] Client disconnected, total: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				go func(c *websocket.Conn, msg *WebSocketMessage) {
					if err := websocket.JSON.Send(c, msg); err != nil {
						log.Printf("[WebSocket] Send error: %v", err)
						h.unregister <- c
					}
				}(client, message)
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 向所有客户端广播快照
func (h *WebSocketHub) Broadcast(snap *DashboardSnapshot) {
	msg := &WebSocketMessage{
		Type:      "dashboard_update",
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      snap,
	}

	select {
	case h.broadcast <- msg:
	default:
		// broadcast channel 满了，跳过（避免阻塞策略 goroutine）
	}
}

// HandleWebSocket 处理 WebSocket 连接升级
func (h *WebSocketHub) HandleWebSocket(ws *websocket.Conn) {
	h.register <- ws

	// 心跳
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-h.stopCh:
				return
			case <-ticker.C:
				if err := websocket.JSON.Send(ws, &WebSocketMessage{
					Type:      "ping",
					Timestamp: time.Now().Format(time.RFC3339),
				}); err != nil {
					h.unregister <- ws
					return
				}
			}
		}
	}()

	// 读循环（处理 pong 和断连检测）
	for {
		var msg map[string]interface{}
		if err := websocket.JSON.Receive(ws, &msg); err != nil {
			h.unregister <- ws
			break
		}
		// pong 或其他客户端消息，忽略
	}
}

// ClientCount 返回当前连接的客户端数
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
