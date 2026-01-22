package indicators

import (
	"fmt"
	"log"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// SharedIndicatorPool manages shared indicator libraries per symbol
// 按symbol管理共享指标库（类似tbsrc的Instrument级指标）
type SharedIndicatorPool struct {
	pools map[string]*IndicatorLibrary // symbol -> shared indicator library
	mu    sync.RWMutex
}

// NewSharedIndicatorPool creates a new shared indicator pool
func NewSharedIndicatorPool() *SharedIndicatorPool {
	return &SharedIndicatorPool{
		pools: make(map[string]*IndicatorLibrary),
	}
}

// GetOrCreate gets or creates a shared indicator library for a symbol
func (sp *SharedIndicatorPool) GetOrCreate(symbol string) *IndicatorLibrary {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if lib, exists := sp.pools[symbol]; exists {
		return lib
	}

	// Create new shared indicator library with common indicators
	lib := NewIndicatorLibrary()

	log.Printf("[SharedIndicatorPool] Created shared indicators for symbol: %s", symbol)
	return lib
}

// InitializeDefaultIndicators initializes default shared indicators for a symbol
// 为symbol初始化默认的共享指标（通用基础指标）
func (sp *SharedIndicatorPool) InitializeDefaultIndicators(symbol string, config map[string]interface{}) error {
	lib := sp.GetOrCreate(symbol)

	// VWAP - Volume Weighted Average Price
	_, err := lib.Create("vwap", "vwap", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to create VWAP: %w", err)
	}

	// Spread - Bid-Ask Spread
	_, err = lib.Create("spread", "spread", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to create Spread: %w", err)
	}

	// Order Imbalance
	_, err = lib.Create("order_imbalance", "order_imbalance", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to create OrderImbalance: %w", err)
	}

	// Volatility
	volConfig := map[string]interface{}{
		"window": 20,
	}
	if cfg, ok := config["volatility"]; ok {
		if cfgMap, ok := cfg.(map[string]interface{}); ok {
			volConfig = cfgMap
		}
	}
	_, err = lib.Create("volatility", "volatility", volConfig)
	if err != nil {
		return fmt.Errorf("failed to create Volatility: %w", err)
	}

	sp.mu.Lock()
	sp.pools[symbol] = lib
	sp.mu.Unlock()

	log.Printf("[SharedIndicatorPool] Initialized default indicators for %s: VWAP, Spread, OrderImbalance, Volatility", symbol)
	return nil
}

// Get gets the shared indicator library for a symbol
func (sp *SharedIndicatorPool) Get(symbol string) (*IndicatorLibrary, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	lib, exists := sp.pools[symbol]
	return lib, exists
}

// UpdateAll updates all shared indicators for a symbol
// 更新某个symbol的所有共享指标（只计算一次，所有策略共享）
func (sp *SharedIndicatorPool) UpdateAll(symbol string, md *mdpb.MarketDataUpdate) {
	sp.mu.RLock()
	lib, exists := sp.pools[symbol]
	sp.mu.RUnlock()

	if !exists {
		return
	}

	// Update all indicators for this symbol (calculated once, shared by all strategies)
	lib.UpdateAll(md)
}

// GetIndicator gets a specific indicator from the shared pool
func (sp *SharedIndicatorPool) GetIndicator(symbol, name string) (Indicator, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	lib, exists := sp.pools[symbol]
	if !exists {
		return nil, false
	}

	return lib.Get(name)
}

// GetAllSymbols returns all symbols in the pool
func (sp *SharedIndicatorPool) GetAllSymbols() []string {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	symbols := make([]string, 0, len(sp.pools))
	for symbol := range sp.pools {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetStats returns statistics about the shared indicator pool
func (sp *SharedIndicatorPool) GetStats() map[string]int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	stats := make(map[string]int)
	for symbol, lib := range sp.pools {
		stats[symbol] = len(lib.indicators)
	}
	return stats
}

// Clear clears all shared indicators (for testing or reset)
func (sp *SharedIndicatorPool) Clear() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.pools = make(map[string]*IndicatorLibrary)
	log.Println("[SharedIndicatorPool] Cleared all shared indicators")
}

// Remove removes shared indicators for a specific symbol
func (sp *SharedIndicatorPool) Remove(symbol string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	delete(sp.pools, symbol)
	log.Printf("[SharedIndicatorPool] Removed shared indicators for symbol: %s", symbol)
}
