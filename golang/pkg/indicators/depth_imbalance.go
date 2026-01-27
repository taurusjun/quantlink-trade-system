package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// DepthImbalance calculates the imbalance between bid and ask depth
// 深度不平衡度：计算买卖盘深度的不平衡程度
// Formula: (BidDepth - AskDepth) / (BidDepth + AskDepth)
// Range: [-1, 1], where 1 = only bids, -1 = only asks, 0 = balanced
type DepthImbalance struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels int // Number of price levels to consider

	// Current state
	bidDepth float64 // Total bid volume
	askDepth float64 // Total ask volume
	imbalance float64 // Current imbalance value
}

// NewDepthImbalance creates a new DepthImbalance indicator
func NewDepthImbalance(name string, levels int, maxHistory int) *DepthImbalance {
	if levels <= 0 {
		levels = 5
	}

	return &DepthImbalance{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
	}
}

// NewDepthImbalanceFromConfig creates a DepthImbalance indicator from configuration
func NewDepthImbalanceFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "DepthImbalance"
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

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewDepthImbalance(name, levels, maxHistory), nil
}

// Update updates the indicator with new market data
func (di *DepthImbalance) Update(md *mdpb.MarketDataUpdate) {
	di.mu.Lock()
	defer di.mu.Unlock()

	// Calculate bid depth
	bidDepth := 0.0
	for i := 0; i < di.levels && i < len(md.BidQty); i++ {
		bidDepth += float64(md.BidQty[i])
	}

	// Calculate ask depth
	askDepth := 0.0
	for i := 0; i < di.levels && i < len(md.AskQty); i++ {
		askDepth += float64(md.AskQty[i])
	}

	di.bidDepth = bidDepth
	di.askDepth = askDepth

	// Calculate imbalance
	totalDepth := bidDepth + askDepth
	if totalDepth > 0 {
		di.imbalance = (bidDepth - askDepth) / totalDepth
	} else {
		di.imbalance = 0
	}

	di.AddValue(di.imbalance)
}

// GetValue returns the current imbalance value
func (di *DepthImbalance) GetValue() float64 {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.imbalance
}

// GetBidDepth returns the current bid depth
func (di *DepthImbalance) GetBidDepth() float64 {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.bidDepth
}

// GetAskDepth returns the current ask depth
func (di *DepthImbalance) GetAskDepth() float64 {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.askDepth
}

// GetImbalancePercentage returns the imbalance as a percentage
func (di *DepthImbalance) GetImbalancePercentage() float64 {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.imbalance * 100.0
}

// IsBidDominant returns true if bids dominate asks
func (di *DepthImbalance) IsBidDominant() bool {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.imbalance > 0
}

// IsAskDominant returns true if asks dominate bids
func (di *DepthImbalance) IsAskDominant() bool {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.imbalance < 0
}

// IsBalanced returns true if the orderbook is relatively balanced
func (di *DepthImbalance) IsBalanced(threshold float64) bool {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return math.Abs(di.imbalance) < threshold
}

// String returns a string representation of the indicator
func (di *DepthImbalance) String() string {
	return fmt.Sprintf("DepthImbalance(levels=%d, imbalance=%.4f)",
		di.levels, di.GetValue())
}
