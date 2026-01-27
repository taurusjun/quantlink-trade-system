package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// OrderFlowImbalance measures the imbalance between buy and sell order flows
// 订单流不平衡：衡量买卖订单流之间的不平衡程度
//
// Imbalance = (BuyVolume - SellVolume) / (BuyVolume + SellVolume)
// Range: [-1, 1]
// Positive = more buying pressure
// Negative = more selling pressure
type OrderFlowImbalance struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowSize int // Number of ticks to accumulate

	// Current state
	buyVolume  float64 // Accumulated buy volume in window
	sellVolume float64 // Accumulated sell volume in window
	tickCount  int     // Number of ticks processed

	// Historical tracking
	lastPrice float64 // Previous price for tick rule
}

// NewOrderFlowImbalance creates a new Order Flow Imbalance indicator
func NewOrderFlowImbalance(name string, windowSize int, maxHistory int) *OrderFlowImbalance {
	if windowSize <= 0 {
		windowSize = 100 // Default window size
	}

	ofi := &OrderFlowImbalance{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		windowSize:    windowSize,
	}

	return ofi
}

// NewOrderFlowImbalanceFromConfig creates an OrderFlowImbalance from configuration
func NewOrderFlowImbalanceFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	windowSize := 100
	if v, ok := config["window_size"]; ok {
		if fv, ok := v.(float64); ok {
			windowSize = int(fv)
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewOrderFlowImbalance(name, windowSize, maxHistory), nil
}

// Update updates the order flow imbalance
func (ofi *OrderFlowImbalance) Update(md *mdpb.MarketDataUpdate) {
	ofi.mu.Lock()
	defer ofi.mu.Unlock()

	// Get current last price
	lastPrice := md.LastPrice
	if lastPrice == 0 {
		ofi.AddValue(0)
		return
	}

	// Classify trade direction using tick rule
	// If price up: buy initiated
	// If price down: sell initiated
	// If price unchanged: use previous classification
	var tradeVolume float64
	isBuy := false

	if ofi.lastPrice > 0 {
		if lastPrice > ofi.lastPrice {
			// Price up = buy
			isBuy = true
			tradeVolume = float64(md.LastVolume)
		} else if lastPrice < ofi.lastPrice {
			// Price down = sell
			isBuy = false
			tradeVolume = float64(md.LastVolume)
		} else {
			// Price unchanged - use bid/ask to classify
			if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
				midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0
				if lastPrice >= midPrice {
					isBuy = true
				}
			}
			tradeVolume = float64(md.LastVolume)
		}
	}

	// Accumulate volumes
	if isBuy {
		ofi.buyVolume += tradeVolume
	} else {
		ofi.sellVolume += tradeVolume
	}

	ofi.tickCount++

	// Reset window when reaching window size
	if ofi.tickCount >= ofi.windowSize {
		ofi.tickCount = 0
		ofi.buyVolume = 0
		ofi.sellVolume = 0
	}

	// Calculate imbalance
	imbalance := ofi.calculateImbalance()
	ofi.lastPrice = lastPrice

	ofi.AddValue(imbalance)
}

// calculateImbalance calculates the order flow imbalance
func (ofi *OrderFlowImbalance) calculateImbalance() float64 {
	totalVolume := ofi.buyVolume + ofi.sellVolume
	if totalVolume == 0 {
		return 0
	}

	return (ofi.buyVolume - ofi.sellVolume) / totalVolume
}

// GetBuyVolume returns accumulated buy volume
func (ofi *OrderFlowImbalance) GetBuyVolume() float64 {
	ofi.mu.RLock()
	defer ofi.mu.RUnlock()
	return ofi.buyVolume
}

// GetSellVolume returns accumulated sell volume
func (ofi *OrderFlowImbalance) GetSellVolume() float64 {
	ofi.mu.RLock()
	defer ofi.mu.RUnlock()
	return ofi.sellVolume
}

// GetImbalance returns current imbalance
func (ofi *OrderFlowImbalance) GetImbalance() float64 {
	ofi.mu.RLock()
	defer ofi.mu.RUnlock()
	return ofi.calculateImbalance()
}

// GetDirection returns the dominant order flow direction
func (ofi *OrderFlowImbalance) GetDirection() string {
	imbalance := ofi.GetImbalance()
	if imbalance > 0.2 {
		return "StrongBuy"
	} else if imbalance > 0.05 {
		return "Buy"
	} else if imbalance < -0.2 {
		return "StrongSell"
	} else if imbalance < -0.05 {
		return "Sell"
	}
	return "Neutral"
}

// GetName returns indicator name
func (ofi *OrderFlowImbalance) GetName() string {
	return ofi.BaseIndicator.GetName()
}

// String returns a string representation
func (ofi *OrderFlowImbalance) String() string {
	return fmt.Sprintf("OrderFlowImbalance(window=%d, imbalance=%.4f, direction=%s, buy=%.0f, sell=%.0f)",
		ofi.windowSize, ofi.GetImbalance(), ofi.GetDirection(), ofi.buyVolume, ofi.sellVolume)
}
