package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// NetOrderFlow measures the cumulative net order flow (buy - sell)
// 净订单流：衡量累计的买卖净订单流
//
// NetFlow = Σ(BuyVolume - SellVolume)
// Tracks the cumulative imbalance over time
type NetOrderFlow struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	resetOnReverse bool // Reset when flow reverses direction

	// State
	cumulativeFlow float64 // Cumulative net flow
	buyFlow        float64 // Total buy flow
	sellFlow       float64 // Total sell flow

	// Historical tracking
	lastPrice     float64 // Previous price for tick rule
	lastDirection int     // Last trade direction: 1=buy, -1=sell, 0=unknown
}

// NewNetOrderFlow creates a new Net Order Flow indicator
func NewNetOrderFlow(name string, resetOnReverse bool, maxHistory int) *NetOrderFlow {
	nof := &NetOrderFlow{
		BaseIndicator:  NewBaseIndicator(name, maxHistory),
		resetOnReverse: resetOnReverse,
	}

	return nof
}

// NewNetOrderFlowFromConfig creates a NetOrderFlow from configuration
func NewNetOrderFlowFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	resetOnReverse := false
	if v, ok := config["reset_on_reverse"]; ok {
		if bv, ok := v.(bool); ok {
			resetOnReverse = bv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewNetOrderFlow(name, resetOnReverse, maxHistory), nil
}

// Update updates the net order flow
func (nof *NetOrderFlow) Update(md *mdpb.MarketDataUpdate) {
	nof.mu.Lock()
	defer nof.mu.Unlock()

	// Get current price
	currentPrice := md.LastPrice
	if currentPrice == 0 {
		nof.AddValue(nof.cumulativeFlow)
		return
	}

	// Classify trade direction
	direction := 0    // 1=buy, -1=sell
	volume := 1.0     // Default unit volume

	if nof.lastPrice > 0 {
		if currentPrice > nof.lastPrice {
			// Uptick: buy
			direction = 1
		} else if currentPrice < nof.lastPrice {
			// Downtick: sell
			direction = -1
		} else {
			// No change: use bid/ask
			if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
				midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0
				if currentPrice >= midPrice {
					direction = 1
				} else {
					direction = -1
				}
			} else {
				// Use last known direction
				direction = nof.lastDirection
			}
		}
	}

	// Check for flow reversal
	if nof.resetOnReverse && nof.lastDirection != 0 && direction != 0 {
		if nof.lastDirection != direction {
			// Flow reversed: reset
			nof.cumulativeFlow = 0
			nof.buyFlow = 0
			nof.sellFlow = 0
		}
	}

	// Update flows
	if direction > 0 {
		nof.buyFlow += volume
		nof.cumulativeFlow += volume
	} else if direction < 0 {
		nof.sellFlow += volume
		nof.cumulativeFlow -= volume
	}

	// Update state
	nof.lastPrice = currentPrice
	if direction != 0 {
		nof.lastDirection = direction
	}

	nof.AddValue(nof.cumulativeFlow)
}

// GetCumulativeFlow returns cumulative net flow
func (nof *NetOrderFlow) GetCumulativeFlow() float64 {
	nof.mu.RLock()
	defer nof.mu.RUnlock()
	return nof.cumulativeFlow
}

// GetBuyFlow returns total buy flow
func (nof *NetOrderFlow) GetBuyFlow() float64 {
	nof.mu.RLock()
	defer nof.mu.RUnlock()
	return nof.buyFlow
}

// GetSellFlow returns total sell flow
func (nof *NetOrderFlow) GetSellFlow() float64 {
	nof.mu.RLock()
	defer nof.mu.RUnlock()
	return nof.sellFlow
}

// GetFlowDirection returns current flow direction
func (nof *NetOrderFlow) GetFlowDirection() string {
	flow := nof.GetCumulativeFlow()
	if flow > 50 {
		return "StrongBuy"
	} else if flow > 10 {
		return "Buy"
	} else if flow < -50 {
		return "StrongSell"
	} else if flow < -10 {
		return "Sell"
	}
	return "Neutral"
}

// GetFlowStrength returns absolute flow strength
func (nof *NetOrderFlow) GetFlowStrength() float64 {
	nof.mu.RLock()
	defer nof.mu.RUnlock()

	if nof.cumulativeFlow < 0 {
		return -nof.cumulativeFlow
	}
	return nof.cumulativeFlow
}

// Reset resets the cumulative flow
func (nof *NetOrderFlow) Reset() {
	nof.mu.Lock()
	defer nof.mu.Unlock()

	nof.cumulativeFlow = 0
	nof.buyFlow = 0
	nof.sellFlow = 0
}

// GetName returns indicator name
func (nof *NetOrderFlow) GetName() string {
	return nof.BaseIndicator.GetName()
}

// String returns a string representation
func (nof *NetOrderFlow) String() string {
	return fmt.Sprintf("NetOrderFlow(cumulative=%.2f, direction=%s, buy=%.0f, sell=%.0f)",
		nof.cumulativeFlow, nof.GetFlowDirection(), nof.buyFlow, nof.sellFlow)
}
