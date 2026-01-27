package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// MarketDepth calculates cumulative depth at multiple price levels
// 多档市场深度：计算不同价格档位的累计挂单量
type MarketDepth struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels int // Number of levels to track

	// Current state
	bidDepths []float64 // Cumulative bid depth at each level
	askDepths []float64 // Cumulative ask depth at each level
	bidPrices []float64 // Bid prices at each level
	askPrices []float64 // Ask prices at each level
}

// NewMarketDepth creates a new Market Depth indicator
func NewMarketDepth(name string, levels int, maxHistory int) *MarketDepth {
	if levels <= 0 {
		levels = 10 // Default to 10 levels
	}

	md := &MarketDepth{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		bidDepths:     make([]float64, levels),
		askDepths:     make([]float64, levels),
		bidPrices:     make([]float64, levels),
		askPrices:     make([]float64, levels),
	}

	return md
}

// NewMarketDepthFromConfig creates a MarketDepth from configuration
func NewMarketDepthFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	levels := 10
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

	return NewMarketDepth(name, levels, maxHistory), nil
}

// Update updates the market depth
func (md *MarketDepth) Update(marketData *mdpb.MarketDataUpdate) {
	md.mu.Lock()
	defer md.mu.Unlock()

	// Reset arrays
	for i := range md.bidDepths {
		md.bidDepths[i] = 0
		md.askDepths[i] = 0
		md.bidPrices[i] = 0
		md.askPrices[i] = 0
	}

	// Calculate cumulative bid depth
	cumBidDepth := 0.0
	for i := 0; i < md.levels && i < len(marketData.BidQty); i++ {
		cumBidDepth += float64(marketData.BidQty[i])
		md.bidDepths[i] = cumBidDepth
		if i < len(marketData.BidPrice) {
			md.bidPrices[i] = marketData.BidPrice[i]
		}
	}

	// Calculate cumulative ask depth
	cumAskDepth := 0.0
	for i := 0; i < md.levels && i < len(marketData.AskQty); i++ {
		cumAskDepth += float64(marketData.AskQty[i])
		md.askDepths[i] = cumAskDepth
		if i < len(marketData.AskPrice) {
			md.askPrices[i] = marketData.AskPrice[i]
		}
	}

	// Store total depth
	totalDepth := cumBidDepth + cumAskDepth
	md.AddValue(totalDepth)
}

// GetBidDepth returns cumulative bid depth at given level
func (md *MarketDepth) GetBidDepth(level int) float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	if level < 0 || level >= len(md.bidDepths) {
		return 0
	}
	return md.bidDepths[level]
}

// GetAskDepth returns cumulative ask depth at given level
func (md *MarketDepth) GetAskDepth(level int) float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	if level < 0 || level >= len(md.askDepths) {
		return 0
	}
	return md.askDepths[level]
}

// GetBidDepths returns all cumulative bid depths
func (md *MarketDepth) GetBidDepths() []float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	result := make([]float64, len(md.bidDepths))
	copy(result, md.bidDepths)
	return result
}

// GetAskDepths returns all cumulative ask depths
func (md *MarketDepth) GetAskDepths() []float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	result := make([]float64, len(md.askDepths))
	copy(result, md.askDepths)
	return result
}

// GetTotalDepth returns total depth (bid + ask)
func (md *MarketDepth) GetTotalDepth() float64 {
	return md.GetValue()
}

// GetDepthImbalance returns depth imbalance: (bid - ask) / total
func (md *MarketDepth) GetDepthImbalance() float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	bidTotal := 0.0
	askTotal := 0.0

	if len(md.bidDepths) > 0 {
		bidTotal = md.bidDepths[len(md.bidDepths)-1]
	}
	if len(md.askDepths) > 0 {
		askTotal = md.askDepths[len(md.askDepths)-1]
	}

	total := bidTotal + askTotal
	if total == 0 {
		return 0
	}

	return (bidTotal - askTotal) / total
}

// GetDepthAtPrice returns available depth up to the given price
// direction: "buy" (uses ask side) or "sell" (uses bid side)
func (md *MarketDepth) GetDepthAtPrice(price float64, direction string) float64 {
	md.mu.RLock()
	defer md.mu.RUnlock()

	if direction == "buy" {
		// Buying: consuming ask side
		for i := 0; i < len(md.askPrices); i++ {
			if md.askPrices[i] > 0 && md.askPrices[i] <= price {
				if i == len(md.askPrices)-1 || md.askPrices[i+1] > price {
					return md.askDepths[i]
				}
			}
		}
		if len(md.askDepths) > 0 {
			return md.askDepths[len(md.askDepths)-1]
		}
	} else if direction == "sell" {
		// Selling: consuming bid side
		for i := 0; i < len(md.bidPrices); i++ {
			if md.bidPrices[i] > 0 && md.bidPrices[i] >= price {
				if i == len(md.bidPrices)-1 || md.bidPrices[i+1] < price {
					return md.bidDepths[i]
				}
			}
		}
		if len(md.bidDepths) > 0 {
			return md.bidDepths[len(md.bidDepths)-1]
		}
	}
	return 0
}

// GetName returns indicator name
func (md *MarketDepth) GetName() string {
	return md.BaseIndicator.GetName()
}

// String returns a string representation
func (md *MarketDepth) String() string {
	md.mu.RLock()
	defer md.mu.RUnlock()

	bidTotal := 0.0
	askTotal := 0.0
	if len(md.bidDepths) > 0 {
		bidTotal = md.bidDepths[len(md.bidDepths)-1]
	}
	if len(md.askDepths) > 0 {
		askTotal = md.askDepths[len(md.askDepths)-1]
	}

	return fmt.Sprintf("MarketDepth(levels=%d, bid=%.0f, ask=%.0f, total=%.0f, imbalance=%.4f)",
		md.levels, bidTotal, askTotal, md.GetValue(), md.GetDepthImbalance())
}
