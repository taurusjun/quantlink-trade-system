package api

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

// Command 从 Web UI 发到 main goroutine 的控制命令
type Command struct {
	Type string // "activate", "deactivate", "squareoff", "reload_thresholds"
}

// Server HTTP + WebSocket 服务
type Server struct {
	hub        *WebSocketHub
	snapshot   atomic.Pointer[DashboardSnapshot]
	cmdChan    chan Command
	httpServer *http.Server
	port       int
	webFS      fs.FS // 静态文件（从外部传入，由 main 包 embed）

	// 订单历史追踪器（每个 leg 各一个）
	leg1History *OrderHistoryTracker
	leg2History *OrderHistoryTracker
}

// NewServer 创建 API Server
// webFS 参数为嵌入的 web/ 静态文件，可以为 nil（不提供前端页面）
func NewServer(port int, webFS fs.FS) *Server {
	return &Server{
		hub:         NewWebSocketHub(),
		cmdChan:     make(chan Command, 10),
		port:        port,
		webFS:       webFS,
		leg1History: NewOrderHistoryTracker(50),
		leg2History: NewOrderHistoryTracker(50),
	}
}

// Start 启动 HTTP server 和 WebSocket hub
func (s *Server) Start() {
	s.hub.Start()

	mux := http.NewServeMux()

	// REST 端点
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/orders", s.handleOrders)
	mux.HandleFunc("POST /api/v1/strategy/activate", s.handleActivate)
	mux.HandleFunc("POST /api/v1/strategy/deactivate", s.handleDeactivate)
	mux.HandleFunc("POST /api/v1/strategy/squareoff", s.handleSquareoff)
	mux.HandleFunc("POST /api/v1/strategy/reload-thresholds", s.handleReloadThresholds)

	// WebSocket
	mux.Handle("/ws", websocket.Handler(s.hub.HandleWebSocket))

	// 静态文件
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		mux.Handle("/", fileServer)
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		log.Printf("[API] Server starting on :%d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[API] Server error: %v", err)
		}
	}()
}

// Stop 优雅关闭
func (s *Server) Stop() {
	s.hub.Stop()
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
		log.Printf("[API] Server stopped")
	}
}

// UpdateSnapshot 原子更新快照并广播到 WebSocket
// 使用 OrderHistoryTracker 合并订单历史，保留已成交/已取消的订单
func (s *Server) UpdateSnapshot(snap *DashboardSnapshot) {
	snap.Leg1.Orders = s.leg1History.Update(snap.Leg1.Orders)
	snap.Leg2.Orders = s.leg2History.Update(snap.Leg2.Orders)
	s.snapshot.Store(snap)
	s.hub.Broadcast(snap)
}

// CommandChan 返回命令 channel，供 main goroutine 读取
func (s *Server) CommandChan() <-chan Command {
	return s.cmdChan
}
