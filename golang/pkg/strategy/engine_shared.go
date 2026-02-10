package strategy

import (
	"log"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
)

// InitializeSharedIndicators initializes shared indicators for a symbol
// 为symbol初始化共享指标
func (se *StrategyEngine) InitializeSharedIndicators(symbol string, config map[string]interface{}) error {
	return se.sharedIndPool.InitializeDefaultIndicators(symbol, config)
}

// GetSharedIndicators gets the shared indicator library for a symbol
// 获取symbol的共享指标库
func (se *StrategyEngine) GetSharedIndicators(symbol string) (*indicators.IndicatorLibrary, bool) {
	return se.sharedIndPool.Get(symbol)
}

// GetOrCreateSharedIndicators gets or creates shared indicators for a symbol
// 获取或创建symbol的共享指标库
func (se *StrategyEngine) GetOrCreateSharedIndicators(symbol string) *indicators.IndicatorLibrary {
	return se.sharedIndPool.GetOrCreate(symbol)
}

// SharedIndicatorAware is an optional interface for strategies that use shared indicators
type SharedIndicatorAware interface {
	SetSharedIndicators(lib *indicators.IndicatorLibrary)
}

// AttachSharedIndicators attaches shared indicators to a strategy
// 将共享指标附加到策略（在AddStrategy时调用）
func (se *StrategyEngine) AttachSharedIndicators(strategy Strategy, symbols []string) error {
	// Check if strategy supports shared indicators
	sharedAware, ok := strategy.(SharedIndicatorAware)
	if !ok {
		// Strategy doesn't need shared indicators, skip
		log.Printf("[StrategyEngine] Strategy %s does not implement SharedIndicatorAware, skipping shared indicators",
			strategy.GetID())
		return nil
	}

	// For now, attach the first symbol's shared indicators
	// In practice, multi-symbol strategies need more complex handling
	if len(symbols) > 0 {
		sharedLib := se.GetOrCreateSharedIndicators(symbols[0])
		sharedAware.SetSharedIndicators(sharedLib)
		log.Printf("[StrategyEngine] Attached shared indicators for %s to strategy %s",
			symbols[0], strategy.GetID())
	}

	return nil
}

// GetSharedIndicatorStats returns statistics about shared indicators
func (se *StrategyEngine) GetSharedIndicatorStats() map[string]int {
	return se.sharedIndPool.GetStats()
}

// RemoveSharedIndicators removes shared indicators for a symbol
func (se *StrategyEngine) RemoveSharedIndicators(symbol string) {
	se.sharedIndPool.Remove(symbol)
}
