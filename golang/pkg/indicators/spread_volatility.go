package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// SpreadVolatility measures the volatility of bid-ask spread
// 价差波动率：衡量买卖价差的波动程度
//
// High volatility = spread changes frequently and dramatically
// Low volatility = spread is stable
type SpreadVolatility struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowSize int  // Number of observations
	normalized bool // Use relative spread (spread/mid) instead of absolute

	// Historical tracking
	spreadHistory []float64 // Recent spread values

	// Current state
	volatility     float64 // Current spread volatility
	avgSpread      float64 // Average spread in window
	minSpread      float64 // Minimum spread in window
	maxSpread      float64 // Maximum spread in window
	spreadRange    float64 // Max - Min spread
	coefficientOfVariation float64 // Volatility / Mean
}

// NewSpreadVolatility creates a new Spread Volatility indicator
func NewSpreadVolatility(name string, windowSize int, normalized bool, maxHistory int) *SpreadVolatility {
	if windowSize <= 0 {
		windowSize = 100
	}

	sv := &SpreadVolatility{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		windowSize:    windowSize,
		normalized:    normalized,
		spreadHistory: make([]float64, 0, windowSize),
	}

	return sv
}

// NewSpreadVolatilityFromConfig creates a SpreadVolatility from configuration
func NewSpreadVolatilityFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "SpreadVolatility"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}

	windowSize := 100
	if v, ok := config["window_size"]; ok {
		if fv, ok := v.(float64); ok {
			windowSize = int(fv)
		}
	}

	normalized := true
	if v, ok := config["normalized"]; ok {
		if bv, ok := v.(bool); ok {
			normalized = bv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewSpreadVolatility(name, windowSize, normalized, maxHistory), nil
}

// Update updates the spread volatility
func (sv *SpreadVolatility) Update(md *mdpb.MarketDataUpdate) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		sv.AddValue(sv.volatility)
		return
	}

	bidPrice := md.BidPrice[0]
	askPrice := md.AskPrice[0]

	if bidPrice == 0 || askPrice == 0 {
		sv.AddValue(sv.volatility)
		return
	}

	// Calculate spread
	spread := askPrice - bidPrice

	// Normalize if configured
	if sv.normalized {
		midPrice := (bidPrice + askPrice) / 2.0
		if midPrice > 0 {
			spread = spread / midPrice
		}
	}

	// Add to history
	sv.spreadHistory = append(sv.spreadHistory, spread)

	// Remove oldest if window is full
	if len(sv.spreadHistory) > sv.windowSize {
		sv.spreadHistory = sv.spreadHistory[1:]
	}

	// Calculate metrics
	sv.calculateMetrics()

	sv.AddValue(sv.volatility)
}

// calculateMetrics calculates volatility and related metrics
func (sv *SpreadVolatility) calculateMetrics() {
	if len(sv.spreadHistory) < 2 {
		sv.volatility = 0
		sv.avgSpread = 0
		sv.minSpread = 0
		sv.maxSpread = 0
		sv.spreadRange = 0
		sv.coefficientOfVariation = 0
		return
	}

	// Calculate average
	sum := 0.0
	min := sv.spreadHistory[0]
	max := sv.spreadHistory[0]

	for _, s := range sv.spreadHistory {
		sum += s
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	sv.avgSpread = sum / float64(len(sv.spreadHistory))
	sv.minSpread = min
	sv.maxSpread = max
	sv.spreadRange = max - min

	// Calculate volatility (standard deviation)
	variance := 0.0
	for _, s := range sv.spreadHistory {
		diff := s - sv.avgSpread
		variance += diff * diff
	}
	variance /= float64(len(sv.spreadHistory))
	sv.volatility = math.Sqrt(variance)

	// Calculate coefficient of variation (CV = std/mean)
	if sv.avgSpread > 0 {
		sv.coefficientOfVariation = sv.volatility / sv.avgSpread
	} else {
		sv.coefficientOfVariation = 0
	}
}

// GetVolatility returns current spread volatility
func (sv *SpreadVolatility) GetVolatility() float64 {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.volatility
}

// GetAvgSpread returns average spread in window
func (sv *SpreadVolatility) GetAvgSpread() float64 {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.avgSpread
}

// GetSpreadRange returns spread range (max - min)
func (sv *SpreadVolatility) GetSpreadRange() float64 {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.spreadRange
}

// GetCoefficientOfVariation returns CV (volatility / mean)
func (sv *SpreadVolatility) GetCoefficientOfVariation() float64 {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.coefficientOfVariation
}

// GetVolatilityLevel returns volatility level classification
func (sv *SpreadVolatility) GetVolatilityLevel() string {
	cv := sv.GetCoefficientOfVariation()

	if cv > 0.5 {
		return "VeryHigh"
	} else if cv > 0.3 {
		return "High"
	} else if cv > 0.15 {
		return "Medium"
	} else if cv > 0.05 {
		return "Low"
	}
	return "VeryLow"
}

// IsStable returns true if spread is relatively stable
func (sv *SpreadVolatility) IsStable() bool {
	return sv.GetCoefficientOfVariation() < 0.15
}

// GetName returns indicator name
func (sv *SpreadVolatility) GetName() string {
	return sv.BaseIndicator.GetName()
}

// String returns a string representation
func (sv *SpreadVolatility) String() string {
	return fmt.Sprintf("SpreadVolatility(volatility=%.6f, avg=%.6f, cv=%.3f, level=%s, range=%.6f)",
		sv.volatility, sv.avgSpread, sv.coefficientOfVariation, sv.GetVolatilityLevel(), sv.spreadRange)
}
