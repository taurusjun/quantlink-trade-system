package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// DepthToSpread measures liquidity quality by comparing depth to spread
// 深度价差比：通过比较深度和价差来衡量流动性质量
//
// Ratio = TotalDepth / Spread
// Higher ratio = better liquidity (more depth, smaller spread)
type DepthToSpread struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels     int     // Number of levels for depth calculation
	normalized bool    // Whether to use relative spread (true) or absolute spread (false)
	minSpread  float64 // Minimum spread to avoid division by zero

	// Current state
	ratio float64 // Current depth-to-spread ratio
}

// NewDepthToSpread creates a new Depth-to-Spread indicator
func NewDepthToSpread(name string, levels int, normalized bool, maxHistory int) *DepthToSpread {
	if levels <= 0 {
		levels = 5
	}

	dts := &DepthToSpread{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		normalized:    normalized,
		minSpread:     0.0001, // Default minimum spread (0.01%)
	}

	return dts
}

// NewDepthToSpreadFromConfig creates a DepthToSpread from configuration
func NewDepthToSpreadFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "DepthToSpread"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}

	levels := 5
	if v, ok := config["levels"]; ok {
		if fv, ok := v.(float64); ok {
			levels = int(fv)
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

	dts := NewDepthToSpread(name, levels, normalized, maxHistory)

	if v, ok := config["min_spread"]; ok {
		if fv, ok := v.(float64); ok {
			dts.minSpread = fv
		}
	}

	return dts, nil
}

// Update updates the depth-to-spread ratio
func (dts *DepthToSpread) Update(md *mdpb.MarketDataUpdate) {
	dts.mu.Lock()
	defer dts.mu.Unlock()

	// Calculate total depth
	totalDepth := 0.0
	for i := 0; i < dts.levels && i < len(md.BidQty); i++ {
		totalDepth += float64(md.BidQty[i])
	}
	for i := 0; i < dts.levels && i < len(md.AskQty); i++ {
		totalDepth += float64(md.AskQty[i])
	}

	// Calculate spread
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		dts.ratio = 0
		dts.AddValue(0)
		return
	}

	bidPrice := md.BidPrice[0]
	askPrice := md.AskPrice[0]

	if bidPrice <= 0 || askPrice <= 0 {
		dts.ratio = 0
		dts.AddValue(0)
		return
	}

	spread := askPrice - bidPrice

	// Normalize spread if configured
	if dts.normalized {
		midPrice := (bidPrice + askPrice) / 2.0
		if midPrice > 0 {
			spread = spread / midPrice
		}
	}

	// Apply minimum spread
	spread = math.Max(spread, dts.minSpread)

	// Calculate ratio
	dts.ratio = totalDepth / spread

	dts.AddValue(dts.ratio)
}

// GetRatio returns the current depth-to-spread ratio
func (dts *DepthToSpread) GetRatio() float64 {
	dts.mu.RLock()
	defer dts.mu.RUnlock()
	return dts.ratio
}

// GetName returns indicator name
func (dts *DepthToSpread) GetName() string {
	return dts.BaseIndicator.GetName()
}

// String returns a string representation
func (dts *DepthToSpread) String() string {
	return fmt.Sprintf("DepthToSpread(levels=%d, normalized=%v, ratio=%.2f)",
		dts.levels, dts.normalized, dts.ratio)
}
