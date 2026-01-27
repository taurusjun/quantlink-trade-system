package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BookDepth calculates the total volume available at multiple price levels
// 订单簿深度：计算多档位的总挂单量
type BookDepth struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels int  // Number of price levels to aggregate
	side   string // "bid", "ask", or "both"

	// Current state
	bidDepth float64 // Total bid volume
	askDepth float64 // Total ask volume
	totalDepth float64 // Total depth (bid + ask)
}

// NewBookDepth creates a new BookDepth indicator
func NewBookDepth(name string, levels int, side string, maxHistory int) *BookDepth {
	if levels <= 0 {
		levels = 5 // Default to 5 levels
	}
	if side == "" {
		side = "both"
	}

	bd := &BookDepth{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		side:          side,
	}

	return bd
}

// NewBookDepthFromConfig creates a BookDepth indicator from configuration
func NewBookDepthFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	levels := 5
	if v, ok := config["levels"]; ok {
		if fv, ok := v.(float64); ok {
			levels = int(fv)
		}
	}

	side := "both"
	if v, ok := config["side"]; ok {
		if sv, ok := v.(string); ok {
			side = sv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewBookDepth(name, levels, side, maxHistory), nil
}

// Update updates the indicator with new market data
func (bd *BookDepth) Update(md *mdpb.MarketDataUpdate) {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	// Calculate bid depth
	bidDepth := 0.0
	for i := 0; i < bd.levels && i < len(md.BidQty); i++ {
		bidDepth += float64(md.BidQty[i])
	}

	// Calculate ask depth
	askDepth := 0.0
	for i := 0; i < bd.levels && i < len(md.AskQty); i++ {
		askDepth += float64(md.AskQty[i])
	}

	bd.bidDepth = bidDepth
	bd.askDepth = askDepth
	bd.totalDepth = bidDepth + askDepth

	// Store value based on side configuration
	var value float64
	switch bd.side {
	case "bid":
		value = bidDepth
	case "ask":
		value = askDepth
	default: // "both"
		value = bd.totalDepth
	}

	bd.AddValue(value)
}

// GetValue returns the current depth value
func (bd *BookDepth) GetValue() float64 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	switch bd.side {
	case "bid":
		return bd.bidDepth
	case "ask":
		return bd.askDepth
	default:
		return bd.totalDepth
	}
}

// GetBidDepth returns the current bid depth
func (bd *BookDepth) GetBidDepth() float64 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.bidDepth
}

// GetAskDepth returns the current ask depth
func (bd *BookDepth) GetAskDepth() float64 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.askDepth
}

// GetTotalDepth returns the total depth (bid + ask)
func (bd *BookDepth) GetTotalDepth() float64 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.totalDepth
}

// String returns a string representation of the indicator
func (bd *BookDepth) String() string {
	return fmt.Sprintf("BookDepth(levels=%d, side=%s, value=%.2f)",
		bd.levels, bd.side, bd.GetValue())
}
