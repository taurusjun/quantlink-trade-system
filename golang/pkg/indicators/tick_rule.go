package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// TickRule classifies trades as buyer-initiated or seller-initiated
// Tick规则：根据价格变化方向分类交易（买方主导或卖方主导）
//
// Rules:
// - Uptick (+1): price > previous price → buyer-initiated
// - Downtick (-1): price < previous price → seller-initiated
// - Zero-uptick (0+): price unchanged, but previous move was up → buyer-initiated
// - Zero-downtick (0-): price unchanged, but previous move was down → seller-initiated
type TickRule struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowSize int // Number of ticks to track

	// State
	lastPrice     float64 // Previous price
	lastTick      int     // Last tick direction: 1=uptick, -1=downtick, 0=unchanged
	currentTick   int     // Current tick direction

	// Statistics in window
	upticks       int // Count of upticks
	downticks     int // Count of downticks
	zeroTicks     int // Count of zero ticks
	tickHistory   []int // Rolling window of tick directions

	// Derived metrics
	tickBalance   float64 // (upticks - downticks) / total
	tickTrend     string  // "Bullish", "Bearish", "Neutral"
}

// NewTickRule creates a new Tick Rule indicator
func NewTickRule(name string, windowSize int, maxHistory int) *TickRule {
	if windowSize <= 0 {
		windowSize = 100
	}

	tr := &TickRule{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		windowSize:    windowSize,
		tickHistory:   make([]int, 0, windowSize),
	}

	return tr
}

// NewTickRuleFromConfig creates a TickRule from configuration
func NewTickRuleFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "TickRule"
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

	return NewTickRule(name, windowSize, maxHistory), nil
}

// Update updates the tick rule classification
func (tr *TickRule) Update(md *mdpb.MarketDataUpdate) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	currentPrice := md.LastPrice
	if currentPrice == 0 {
		tr.AddValue(float64(tr.currentTick))
		return
	}

	// Classify current tick
	if tr.lastPrice == 0 {
		// First tick: cannot classify
		tr.lastPrice = currentPrice
		tr.AddValue(0)
		return
	}

	if currentPrice > tr.lastPrice {
		// Uptick
		tr.currentTick = 1
		tr.lastTick = 1
	} else if currentPrice < tr.lastPrice {
		// Downtick
		tr.currentTick = -1
		tr.lastTick = -1
	} else {
		// Zero tick: use previous tick direction
		tr.currentTick = tr.lastTick
	}

	// Update statistics
	tr.addTick(tr.currentTick)

	// Calculate tick balance
	total := len(tr.tickHistory)
	if total > 0 {
		tr.tickBalance = float64(tr.upticks-tr.downticks) / float64(total)
	}

	// Determine trend
	tr.updateTrend()

	// Update last price
	tr.lastPrice = currentPrice

	// Store current tick as value
	tr.AddValue(float64(tr.currentTick))
}

// addTick adds a tick to the rolling window
func (tr *TickRule) addTick(tick int) {
	tr.tickHistory = append(tr.tickHistory, tick)

	// Update counts
	switch tick {
	case 1:
		tr.upticks++
	case -1:
		tr.downticks++
	case 0:
		tr.zeroTicks++
	}

	// Remove oldest tick if window is full
	if len(tr.tickHistory) > tr.windowSize {
		oldTick := tr.tickHistory[0]
		tr.tickHistory = tr.tickHistory[1:]

		switch oldTick {
		case 1:
			tr.upticks--
		case -1:
			tr.downticks--
		case 0:
			tr.zeroTicks--
		}
	}
}

// updateTrend updates the tick trend classification
func (tr *TickRule) updateTrend() {
	if tr.tickBalance > 0.2 {
		tr.tickTrend = "Bullish"
	} else if tr.tickBalance < -0.2 {
		tr.tickTrend = "Bearish"
	} else {
		tr.tickTrend = "Neutral"
	}
}

// GetCurrentTick returns the current tick direction
func (tr *TickRule) GetCurrentTick() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.currentTick
}

// GetTickBalance returns the tick balance ratio
func (tr *TickRule) GetTickBalance() float64 {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.tickBalance
}

// GetTrend returns the current tick trend
func (tr *TickRule) GetTrend() string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.tickTrend
}

// GetUptickRatio returns the ratio of upticks
func (tr *TickRule) GetUptickRatio() float64 {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	total := len(tr.tickHistory)
	if total == 0 {
		return 0
	}
	return float64(tr.upticks) / float64(total)
}

// GetDowntickRatio returns the ratio of downticks
func (tr *TickRule) GetDowntickRatio() float64 {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	total := len(tr.tickHistory)
	if total == 0 {
		return 0
	}
	return float64(tr.downticks) / float64(total)
}

// IsBuyerInitiated returns true if current trade is buyer-initiated
func (tr *TickRule) IsBuyerInitiated() bool {
	return tr.GetCurrentTick() > 0
}

// GetName returns indicator name
func (tr *TickRule) GetName() string {
	return tr.BaseIndicator.GetName()
}

// String returns a string representation
func (tr *TickRule) String() string {
	return fmt.Sprintf("TickRule(current=%d, balance=%.3f, trend=%s, up=%d, down=%d, zero=%d)",
		tr.currentTick, tr.tickBalance, tr.tickTrend, tr.upticks, tr.downticks, tr.zeroTicks)
}
