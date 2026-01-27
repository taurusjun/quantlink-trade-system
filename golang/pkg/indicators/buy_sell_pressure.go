package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BuySellPressure measures the pressure from buyers vs sellers
// 买卖压力：衡量买方和卖方的压力强度
//
// Combines orderbook depth and trade flow to measure pressure
// Positive = buying pressure dominant
// Negative = selling pressure dominant
type BuySellPressure struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels       int     // Number of orderbook levels
	depthWeight  float64 // Weight for depth component
	flowWeight   float64 // Weight for order flow component
	decayFactor  float64 // Exponential decay for historical pressure

	// State
	buyPressure  float64 // Current buy pressure
	sellPressure float64 // Current sell pressure
	netPressure  float64 // Net pressure (buy - sell)

	// Historical tracking
	prevBuyVolume  float64 // Previous cumulative buy volume
	prevSellVolume float64 // Previous cumulative sell volume
}

// NewBuySellPressure creates a new Buy-Sell Pressure indicator
func NewBuySellPressure(name string, levels int, maxHistory int) *BuySellPressure {
	if levels <= 0 {
		levels = 5
	}

	bsp := &BuySellPressure{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		depthWeight:   0.7, // Depth contributes 70%
		flowWeight:    0.3, // Flow contributes 30%
		decayFactor:   0.95,
	}

	return bsp
}

// NewBuySellPressureFromConfig creates a BuySellPressure from configuration
func NewBuySellPressureFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "BuySellPressure"
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

	bsp := NewBuySellPressure(name, levels, maxHistory)

	if v, ok := config["depth_weight"]; ok {
		if fv, ok := v.(float64); ok {
			bsp.depthWeight = fv
		}
	}
	if v, ok := config["flow_weight"]; ok {
		if fv, ok := v.(float64); ok {
			bsp.flowWeight = fv
		}
	}
	if v, ok := config["decay_factor"]; ok {
		if fv, ok := v.(float64); ok {
			bsp.decayFactor = fv
		}
	}

	// Normalize weights
	total := bsp.depthWeight + bsp.flowWeight
	if total > 0 {
		bsp.depthWeight /= total
		bsp.flowWeight /= total
	}

	return bsp, nil
}

// Update updates the buy-sell pressure
func (bsp *BuySellPressure) Update(md *mdpb.MarketDataUpdate) {
	bsp.mu.Lock()
	defer bsp.mu.Unlock()

	// Apply decay to historical pressure
	bsp.buyPressure *= bsp.decayFactor
	bsp.sellPressure *= bsp.decayFactor

	// 1. Calculate depth-based pressure
	depthBuyPressure, depthSellPressure := bsp.calculateDepthPressure(md)

	// 2. Calculate flow-based pressure (from trade direction)
	flowBuyPressure, flowSellPressure := bsp.calculateFlowPressure(md)

	// 3. Combine pressures with weights
	bsp.buyPressure += bsp.depthWeight*depthBuyPressure + bsp.flowWeight*flowBuyPressure
	bsp.sellPressure += bsp.depthWeight*depthSellPressure + bsp.flowWeight*flowSellPressure

	// 4. Calculate net pressure
	bsp.netPressure = bsp.buyPressure - bsp.sellPressure

	bsp.AddValue(bsp.netPressure)
}

// calculateDepthPressure calculates pressure from orderbook depth
func (bsp *BuySellPressure) calculateDepthPressure(md *mdpb.MarketDataUpdate) (float64, float64) {
	bidDepth := 0.0
	askDepth := 0.0

	// Weighted depth (closer levels have more weight)
	for i := 0; i < bsp.levels && i < len(md.BidQty); i++ {
		weight := 1.0 / float64(i+1) // Level 0 has weight 1, level 1 has weight 0.5, etc.
		bidDepth += float64(md.BidQty[i]) * weight
	}

	for i := 0; i < bsp.levels && i < len(md.AskQty); i++ {
		weight := 1.0 / float64(i+1)
		askDepth += float64(md.AskQty[i]) * weight
	}

	return bidDepth, askDepth
}

// calculateFlowPressure calculates pressure from order flow
func (bsp *BuySellPressure) calculateFlowPressure(md *mdpb.MarketDataUpdate) (float64, float64) {
	// Use trade direction and volume to infer pressure
	// If last price > mid price: buying pressure
	// If last price < mid price: selling pressure

	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return 0, 0
	}

	midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0
	lastPrice := md.LastPrice

	if lastPrice == 0 {
		return 0, 0
	}

	// Determine trade direction
	volume := 1.0 // Default unit volume if not available

	if lastPrice > midPrice {
		// Buy pressure
		return volume, 0
	} else if lastPrice < midPrice {
		// Sell pressure
		return 0, volume
	}

	// At mid price: split pressure
	return volume * 0.5, volume * 0.5
}

// GetBuyPressure returns current buy pressure
func (bsp *BuySellPressure) GetBuyPressure() float64 {
	bsp.mu.RLock()
	defer bsp.mu.RUnlock()
	return bsp.buyPressure
}

// GetSellPressure returns current sell pressure
func (bsp *BuySellPressure) GetSellPressure() float64 {
	bsp.mu.RLock()
	defer bsp.mu.RUnlock()
	return bsp.sellPressure
}

// GetNetPressure returns net pressure (buy - sell)
func (bsp *BuySellPressure) GetNetPressure() float64 {
	bsp.mu.RLock()
	defer bsp.mu.RUnlock()
	return bsp.netPressure
}

// GetPressureRatio returns buy/sell pressure ratio
func (bsp *BuySellPressure) GetPressureRatio() float64 {
	bsp.mu.RLock()
	defer bsp.mu.RUnlock()

	if bsp.sellPressure == 0 {
		if bsp.buyPressure > 0 {
			return math.Inf(1)
		}
		return 1.0
	}

	return bsp.buyPressure / bsp.sellPressure
}

// GetDominantSide returns which side has dominant pressure
func (bsp *BuySellPressure) GetDominantSide() string {
	net := bsp.GetNetPressure()
	if net > 10 {
		return "StrongBuy"
	} else if net > 2 {
		return "Buy"
	} else if net < -10 {
		return "StrongSell"
	} else if net < -2 {
		return "Sell"
	}
	return "Balanced"
}

// GetName returns indicator name
func (bsp *BuySellPressure) GetName() string {
	return bsp.BaseIndicator.GetName()
}

// String returns a string representation
func (bsp *BuySellPressure) String() string {
	return fmt.Sprintf("BuySellPressure(net=%.2f, buy=%.2f, sell=%.2f, side=%s, ratio=%.2f)",
		bsp.netPressure, bsp.buyPressure, bsp.sellPressure, bsp.GetDominantSide(), bsp.GetPressureRatio())
}
