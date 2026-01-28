package trader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// setupTestTraderWithMultiStrategy creates a test trader in multi-strategy mode
func setupTestTraderWithMultiStrategy(t *testing.T) *Trader {
	cfg := &config.TraderConfig{
		System: config.SystemConfig{
			Mode:          "simulation",
			MultiStrategy: true,
		},
		Strategies: []config.StrategyItemConfig{
			{
				ID:         "test_passive",
				Type:       "passive",
				Enabled:    true,
				Symbols:    []string{"ag2502"},
				Allocation: 0.5,
			},
			{
				ID:         "test_pairwise",
				Type:       "pairwise_arb",
				Enabled:    true,
				Symbols:    []string{"ag2502", "ag2504"},
				Allocation: 0.5,
			},
		},
		Session: config.SessionConfig{
			AutoActivate: false,
		},
		Risk: config.RiskConfig{
			CheckIntervalMs: 100,
		},
		API: config.APIConfig{
			Enabled: true,
			Port:    9999,
		},
	}

	trader, err := NewTrader(cfg)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Create strategy manager manually for testing
	trader.StrategyMgr = strategy.NewStrategyManager(nil)
	if err := trader.StrategyMgr.LoadStrategies(cfg.Strategies); err != nil {
		t.Fatalf("Failed to load strategies: %v", err)
	}

	// Set first strategy for backward compatibility
	trader.Strategy = trader.StrategyMgr.GetFirstStrategy()

	return trader
}

func TestAPIDashboardOverview(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil)
	w := httptest.NewRecorder()

	api.handleDashboardOverview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success true, got false")
	}

	data, ok := response.Data.(*DashboardOverview)
	if !ok {
		// Try map conversion
		dataMap, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Unexpected response data type")
		}

		if dataMap["multi_strategy"] != true {
			t.Errorf("Expected multi_strategy true, got %v", dataMap["multi_strategy"])
		}

		if dataMap["total_strategies"].(float64) != 2 {
			t.Errorf("Expected 2 strategies, got %v", dataMap["total_strategies"])
		}
		return
	}

	if !data.MultiStrategy {
		t.Error("Expected multi_strategy true")
	}

	if data.TotalStrategies != 2 {
		t.Errorf("Expected 2 strategies, got %d", data.TotalStrategies)
	}
}

func TestAPIStrategiesList(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/strategies", nil)
	w := httptest.NewRecorder()

	api.handleStrategies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success true, got false")
	}

	dataMap, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Unexpected response data type")
	}

	if dataMap["count"].(float64) != 2 {
		t.Errorf("Expected count 2, got %v", dataMap["count"])
	}

	strategies, ok := dataMap["strategies"].([]interface{})
	if !ok {
		t.Fatal("Expected strategies array")
	}

	if len(strategies) != 2 {
		t.Errorf("Expected 2 strategies in list, got %d", len(strategies))
	}
}

func TestAPIGetStrategyByID(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	// Test existing strategy
	req := httptest.NewRequest(http.MethodGet, "/api/v1/strategies/test_passive", nil)
	w := httptest.NewRecorder()

	api.handleGetStrategy(w, req, "test_passive")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success true, got false")
	}

	dataMap, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Unexpected response data type")
	}

	if dataMap["id"] != "test_passive" {
		t.Errorf("Expected id 'test_passive', got %v", dataMap["id"])
	}
}

func TestAPIGetNonExistentStrategy(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/strategies/nonexistent", nil)
	w := httptest.NewRecorder()

	api.handleGetStrategy(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIActivateDeactivateStrategy(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	// Activate strategy
	reqActivate := httptest.NewRequest(http.MethodPost, "/api/v1/strategies/test_passive/activate", nil)
	wActivate := httptest.NewRecorder()

	api.handleActivateStrategy(wActivate, reqActivate, "test_passive")

	if wActivate.Code != http.StatusOK {
		t.Errorf("Activate: Expected status 200, got %d", wActivate.Code)
	}

	var activateResp APIResponse
	if err := json.NewDecoder(wActivate.Body).Decode(&activateResp); err != nil {
		t.Fatalf("Failed to decode activate response: %v", err)
	}

	if !activateResp.Success {
		t.Errorf("Activate: Expected success true")
	}

	// Deactivate strategy
	reqDeactivate := httptest.NewRequest(http.MethodPost, "/api/v1/strategies/test_passive/deactivate", nil)
	wDeactivate := httptest.NewRecorder()

	api.handleDeactivateStrategy(wDeactivate, reqDeactivate, "test_passive")

	if wDeactivate.Code != http.StatusOK {
		t.Errorf("Deactivate: Expected status 200, got %d", wDeactivate.Code)
	}

	var deactivateResp APIResponse
	if err := json.NewDecoder(wDeactivate.Body).Decode(&deactivateResp); err != nil {
		t.Fatalf("Failed to decode deactivate response: %v", err)
	}

	if !deactivateResp.Success {
		t.Errorf("Deactivate: Expected success true")
	}
}

func TestAPIRealtimeIndicators(t *testing.T) {
	trader := setupTestTraderWithMultiStrategy(t)
	api := NewAPIServer(trader, 9999)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/indicators/realtime", nil)
	w := httptest.NewRecorder()

	api.handleRealtimeIndicators(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success true, got false")
	}

	dataMap, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Unexpected response data type")
	}

	if _, hasTimestamp := dataMap["timestamp"]; !hasTimestamp {
		t.Error("Expected timestamp field")
	}

	strategies, ok := dataMap["strategies"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected strategies map")
	}

	if len(strategies) != 2 {
		t.Errorf("Expected 2 strategies in indicators, got %d", len(strategies))
	}
}

func TestAPIDashboardOverviewSingleStrategy(t *testing.T) {
	// Test single-strategy mode
	cfg := &config.TraderConfig{
		System: config.SystemConfig{
			StrategyID:    "single_test",
			Mode:          "simulation",
			MultiStrategy: false,
		},
		Strategy: config.StrategyConfig{
			Type:    "passive",
			Symbols: []string{"ag2502"},
		},
		Session: config.SessionConfig{
			AutoActivate: false,
		},
		Risk: config.RiskConfig{
			CheckIntervalMs: 100,
		},
		API: config.APIConfig{
			Enabled: true,
			Port:    9998,
		},
	}

	trader, err := NewTrader(cfg)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Create single strategy
	trader.Strategy = strategy.NewPassiveStrategy("single_test")
	trader.Strategy.Initialize(&strategy.StrategyConfig{
		StrategyID:   "single_test",
		StrategyType: "passive",
		Symbols:      []string{"ag2502"},
	})

	api := NewAPIServer(trader, 9998)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil)
	w := httptest.NewRecorder()

	api.handleDashboardOverview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	dataMap, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Unexpected response data type")
	}

	if dataMap["multi_strategy"] != false {
		t.Error("Expected multi_strategy false for single strategy mode")
	}

	if dataMap["total_strategies"].(float64) != 1 {
		t.Errorf("Expected 1 strategy, got %v", dataMap["total_strategies"])
	}
}
