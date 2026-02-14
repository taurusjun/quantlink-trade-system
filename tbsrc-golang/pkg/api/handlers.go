package api

import (
	"encoding/json"
	"net/http"
)

// jsonResponse 通用 JSON 响应
type jsonResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp jsonResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// GET /api/v1/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Message: "ok",
		Data: map[string]interface{}{
			"ws_clients": s.hub.ClientCount(),
		},
	})
}

// GET /api/v1/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot.Load()
	if snap == nil {
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Message: "no snapshot yet",
		})
		return
	}
	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Data:    snap,
	})
}

// GET /api/v1/orders
func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot.Load()
	if snap == nil {
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Data:    map[string]interface{}{"leg1": []interface{}{}, "leg2": []interface{}{}},
		})
		return
	}
	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Data: map[string]interface{}{
			"leg1": snap.Leg1.Orders,
			"leg2": snap.Leg2.Orders,
		},
	})
}

// POST /api/v1/strategy/activate — 对应 kill -10 (SIGUSR1)
func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	select {
	case s.cmdChan <- Command{Type: "activate"}:
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Message: "activate command sent",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{
			Success: false,
			Message: "command channel full",
		})
	}
}

// POST /api/v1/strategy/deactivate — 停用策略（不平仓）
func (s *Server) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	select {
	case s.cmdChan <- Command{Type: "deactivate"}:
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Message: "deactivate command sent",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{
			Success: false,
			Message: "command channel full",
		})
	}
}

// POST /api/v1/strategy/squareoff — 对应 kill -20 (SIGTSTP)
func (s *Server) handleSquareoff(w http.ResponseWriter, r *http.Request) {
	select {
	case s.cmdChan <- Command{Type: "squareoff"}:
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Message: "squareoff command sent",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{
			Success: false,
			Message: "command channel full",
		})
	}
}

// POST /api/v1/strategy/reload-thresholds — 对应 kill -12 (SIGUSR2)
func (s *Server) handleReloadThresholds(w http.ResponseWriter, r *http.Request) {
	select {
	case s.cmdChan <- Command{Type: "reload_thresholds"}:
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Message: "reload_thresholds command sent",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{
			Success: false,
			Message: "command channel full",
		})
	}
}
