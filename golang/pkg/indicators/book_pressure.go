package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BookPressure calculates the pressure in the orderbook
// 订单簿压力：衡量订单簿中买卖力量的压力程度
// Positive pressure = buying pressure, Negative = selling pressure
type BookPressure struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels        int     // Number of price levels to consider
	weightDecay   float64 // Weight decay for deeper levels (0-1)
	useVolume     bool    // Use volume weighting vs simple count

	// Current state
	buyPressure   float64 // Buying pressure
	sellPressure  float64 // Selling pressure
	netPressure   float64 // Net pressure (buy - sell)
	pressureRatio float64 // Pressure ratio (buy / sell)
}

// NewBookPressure creates a new BookPressure indicator
func NewBookPressure(name string, levels int, weightDecay float64, useVolume bool, maxHistory int) *BookPressure {
	if levels <= 0 {
		levels = 5
	}
	if weightDecay <= 0 || weightDecay > 1 {
		weightDecay = 0.9 // Default decay factor
	}

	return &BookPressure{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		weightDecay:   weightDecay,
		useVolume:     useVolume,
	}
}

// NewBookPressureFromConfig creates a BookPressure indicator from configuration
func NewBookPressureFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "BookPressure"
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

	weightDecay := 0.9
	if v, ok := config["weight_decay"]; ok {
		if fv, ok := v.(float64); ok {
			weightDecay = fv
		}
	}

	useVolume := true
	if v, ok := config["use_volume"]; ok {
		if bv, ok := v.(bool); ok {
			useVolume = bv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewBookPressure(name, levels, weightDecay, useVolume, maxHistory), nil
}

// Update updates the indicator with new market data
func (bp *BookPressure) Update(md *mdpb.MarketDataUpdate) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Calculate weighted buy pressure (bid side)
	buyPressure := 0.0
	weight := 1.0
	for i := 0; i < bp.levels && i < len(md.BidQty); i++ {
		if bp.useVolume {
			buyPressure += float64(md.BidQty[i]) * weight
		} else {
			buyPressure += weight
		}
		weight *= bp.weightDecay
	}

	// Calculate weighted sell pressure (ask side)
	sellPressure := 0.0
	weight = 1.0
	for i := 0; i < bp.levels && i < len(md.AskQty); i++ {
		if bp.useVolume {
			sellPressure += float64(md.AskQty[i]) * weight
		} else {
			sellPressure += weight
		}
		weight *= bp.weightDecay
	}

	bp.buyPressure = buyPressure
	bp.sellPressure = sellPressure
	bp.netPressure = buyPressure - sellPressure

	// Calculate pressure ratio
	if sellPressure > 0 {
		bp.pressureRatio = buyPressure / sellPressure
	} else if buyPressure > 0 {
		bp.pressureRatio = 999.0 // Large positive value
	} else {
		bp.pressureRatio = 1.0 // Neutral
	}

	bp.AddValue(bp.netPressure)
}

// GetValue returns the current net pressure
func (bp *BookPressure) GetValue() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.netPressure
}

// GetBuyPressure returns the buying pressure
func (bp *BookPressure) GetBuyPressure() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.buyPressure
}

// GetSellPressure returns the selling pressure
func (bp *BookPressure) GetSellPressure() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.sellPressure
}

// GetPressureRatio returns the pressure ratio (buy / sell)
func (bp *BookPressure) GetPressureRatio() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.pressureRatio
}

// GetNormalizedPressure returns pressure normalized to [-1, 1]
func (bp *BookPressure) GetNormalizedPressure() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	total := bp.buyPressure + bp.sellPressure
	if total > 0 {
		return bp.netPressure / total
	}
	return 0
}

// IsBuyingPressure returns true if buying pressure dominates
func (bp *BookPressure) IsBuyingPressure() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.netPressure > 0
}

// IsSellingPressure returns true if selling pressure dominates
func (bp *BookPressure) IsSellingPressure() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.netPressure < 0
}

// GetPressureStrength returns the strength of pressure (0-1)
func (bp *BookPressure) GetPressureStrength() float64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	total := bp.buyPressure + bp.sellPressure
	if total > 0 {
		return math.Abs(bp.netPressure) / total
	}
	return 0
}

// String returns a string representation of the indicator
func (bp *BookPressure) String() string {
	return fmt.Sprintf("BookPressure(levels=%d, net=%.2f, ratio=%.2f)",
		bp.levels, bp.GetValue(), bp.GetPressureRatio())
}
