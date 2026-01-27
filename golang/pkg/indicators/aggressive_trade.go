package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// AggressiveTrade measures the ratio of aggressive trades (market takers)
// 主动成交比例：衡量主动成交（市价单）的比例
//
// Aggressive trades are those that cross the spread:
// - Aggressive buy: trade at ask price or higher
// - Aggressive sell: trade at bid price or lower
//
// Ratio = AggressiveTrades / TotalTrades
// Higher ratio = more urgent trading activity
type AggressiveTrade struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowSize    int     // Number of trades to track
	spreadBuffer  float64 // Buffer zone around spread (in % of spread)

	// Tracking
	aggressiveBuyCount  int // Count of aggressive buy trades
	aggressiveSellCount int // Count of aggressive sell trades
	totalTrades         int // Total trades in window

	// Rolling window
	isAggressiveBuy  []bool // Track if each trade was aggressive buy
	isAggressiveSell []bool // Track if each trade was aggressive sell

	// Current state
	aggressiveRatio     float64 // Ratio of aggressive trades
	buyAggressiveRatio  float64 // Ratio of aggressive buys
	sellAggressiveRatio float64 // Ratio of aggressive sells
}

// NewAggressiveTrade creates a new Aggressive Trade indicator
func NewAggressiveTrade(name string, windowSize int, maxHistory int) *AggressiveTrade {
	if windowSize <= 0 {
		windowSize = 100
	}

	at := &AggressiveTrade{
		BaseIndicator:    NewBaseIndicator(name, maxHistory),
		windowSize:       windowSize,
		spreadBuffer:     0.1, // 10% buffer
		isAggressiveBuy:  make([]bool, 0, windowSize),
		isAggressiveSell: make([]bool, 0, windowSize),
	}

	return at
}

// NewAggressiveTradeFromConfig creates an AggressiveTrade from configuration
func NewAggressiveTradeFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "AggressiveTrade"
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

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	at := NewAggressiveTrade(name, windowSize, maxHistory)

	if v, ok := config["spread_buffer"]; ok {
		if fv, ok := v.(float64); ok {
			at.spreadBuffer = fv
		}
	}

	return at, nil
}

// Update updates the aggressive trade ratio
func (at *AggressiveTrade) Update(md *mdpb.MarketDataUpdate) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		at.AddValue(at.aggressiveRatio)
		return
	}

	bidPrice := md.BidPrice[0]
	askPrice := md.AskPrice[0]
	lastPrice := md.LastPrice

	if lastPrice == 0 || bidPrice == 0 || askPrice == 0 {
		at.AddValue(at.aggressiveRatio)
		return
	}

	// Calculate spread and buffer zone
	spread := askPrice - bidPrice
	buffer := spread * at.spreadBuffer

	// Classify trade as aggressive or passive
	isAggBuy := false
	isAggSell := false

	// Aggressive buy: trade at or above ask - buffer
	if lastPrice >= askPrice-buffer {
		isAggBuy = true
		at.aggressiveBuyCount++
	}

	// Aggressive sell: trade at or below bid + buffer
	if lastPrice <= bidPrice+buffer {
		isAggSell = true
		at.aggressiveSellCount++
	}

	// Add to rolling window
	at.isAggressiveBuy = append(at.isAggressiveBuy, isAggBuy)
	at.isAggressiveSell = append(at.isAggressiveSell, isAggSell)
	at.totalTrades++

	// Remove oldest trade if window is full
	if len(at.isAggressiveBuy) > at.windowSize {
		if at.isAggressiveBuy[0] {
			at.aggressiveBuyCount--
		}
		if at.isAggressiveSell[0] {
			at.aggressiveSellCount--
		}
		at.isAggressiveBuy = at.isAggressiveBuy[1:]
		at.isAggressiveSell = at.isAggressiveSell[1:]
		at.totalTrades--
	}

	// Calculate ratios
	if at.totalTrades > 0 {
		totalAggressive := at.aggressiveBuyCount + at.aggressiveSellCount
		at.aggressiveRatio = float64(totalAggressive) / float64(at.totalTrades)
		at.buyAggressiveRatio = float64(at.aggressiveBuyCount) / float64(at.totalTrades)
		at.sellAggressiveRatio = float64(at.aggressiveSellCount) / float64(at.totalTrades)
	}

	at.AddValue(at.aggressiveRatio)
}

// GetAggressiveRatio returns overall aggressive trade ratio
func (at *AggressiveTrade) GetAggressiveRatio() float64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.aggressiveRatio
}

// GetBuyAggressiveRatio returns aggressive buy ratio
func (at *AggressiveTrade) GetBuyAggressiveRatio() float64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.buyAggressiveRatio
}

// GetSellAggressiveRatio returns aggressive sell ratio
func (at *AggressiveTrade) GetSellAggressiveRatio() float64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.sellAggressiveRatio
}

// GetDominantSide returns which side has more aggressive trades
func (at *AggressiveTrade) GetDominantSide() string {
	at.mu.RLock()
	defer at.mu.RUnlock()

	if at.buyAggressiveRatio > at.sellAggressiveRatio*1.5 {
		return "StrongBuy"
	} else if at.buyAggressiveRatio > at.sellAggressiveRatio*1.2 {
		return "Buy"
	} else if at.sellAggressiveRatio > at.buyAggressiveRatio*1.5 {
		return "StrongSell"
	} else if at.sellAggressiveRatio > at.buyAggressiveRatio*1.2 {
		return "Sell"
	}
	return "Balanced"
}

// GetUrgencyLevel returns trading urgency level
func (at *AggressiveTrade) GetUrgencyLevel() string {
	ratio := at.GetAggressiveRatio()
	if ratio > 0.8 {
		return "VeryHigh" // >80% aggressive
	} else if ratio > 0.6 {
		return "High" // 60-80% aggressive
	} else if ratio > 0.4 {
		return "Medium" // 40-60% aggressive
	} else if ratio > 0.2 {
		return "Low" // 20-40% aggressive
	}
	return "VeryLow" // <20% aggressive
}

// GetName returns indicator name
func (at *AggressiveTrade) GetName() string {
	return at.BaseIndicator.GetName()
}

// String returns a string representation
func (at *AggressiveTrade) String() string {
	return fmt.Sprintf("AggressiveTrade(ratio=%.2f, buy=%.2f, sell=%.2f, side=%s, urgency=%s)",
		at.aggressiveRatio, at.buyAggressiveRatio, at.sellAggressiveRatio,
		at.GetDominantSide(), at.GetUrgencyLevel())
}
