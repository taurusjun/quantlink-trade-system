package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// QuoteSlope measures the price impact curve (how price changes with depth)
// 报价斜率：衡量价格冲击曲线，即价格随深度变化的速度
//
// Slope = ΔPrice / ΔDepth
// Higher slope = steeper orderbook = lower liquidity
type QuoteSlope struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels int // Number of levels to analyze

	// Current state
	bidSlope float64 // Bid side slope
	askSlope float64 // Ask side slope
	avgSlope float64 // Average slope
}

// NewQuoteSlope creates a new Quote Slope indicator
func NewQuoteSlope(name string, levels int, maxHistory int) *QuoteSlope {
	if levels <= 0 {
		levels = 5
	}

	qs := &QuoteSlope{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
	}

	return qs
}

// NewQuoteSlopeFromConfig creates a QuoteSlope from configuration
func NewQuoteSlopeFromConfig(name string, config map[string]interface{}) (Indicator, error) {
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

	return NewQuoteSlope(name, levels, maxHistory), nil
}

// Update updates the quote slope
func (qs *QuoteSlope) Update(md *mdpb.MarketDataUpdate) {
	qs.mu.Lock()
	defer qs.mu.Unlock()

	// Calculate bid slope
	qs.bidSlope = qs.calculateBidSlope(md)

	// Calculate ask slope
	qs.askSlope = qs.calculateAskSlope(md)

	// Average slope
	qs.avgSlope = (math.Abs(qs.bidSlope) + math.Abs(qs.askSlope)) / 2.0

	qs.AddValue(qs.avgSlope)
}

// calculateBidSlope calculates bid side slope
func (qs *QuoteSlope) calculateBidSlope(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidPrice) < 2 || len(md.BidQty) < 2 {
		return 0
	}

	// Calculate cumulative depth and price change
	cumDepth := 0.0
	firstPrice := md.BidPrice[0]
	lastPrice := firstPrice
	lastDepth := 0.0

	for i := 0; i < qs.levels && i < len(md.BidQty); i++ {
		cumDepth += float64(md.BidQty[i])
		if i < len(md.BidPrice) {
			lastPrice = md.BidPrice[i]
			lastDepth = cumDepth
		}
	}

	if lastDepth == 0 {
		return 0
	}

	// Slope = price change / depth
	priceChange := math.Abs(firstPrice - lastPrice)
	slope := priceChange / lastDepth

	return slope
}

// calculateAskSlope calculates ask side slope
func (qs *QuoteSlope) calculateAskSlope(md *mdpb.MarketDataUpdate) float64 {
	if len(md.AskPrice) < 2 || len(md.AskQty) < 2 {
		return 0
	}

	// Calculate cumulative depth and price change
	cumDepth := 0.0
	firstPrice := md.AskPrice[0]
	lastPrice := firstPrice
	lastDepth := 0.0

	for i := 0; i < qs.levels && i < len(md.AskQty); i++ {
		cumDepth += float64(md.AskQty[i])
		if i < len(md.AskPrice) {
			lastPrice = md.AskPrice[i]
			lastDepth = cumDepth
		}
	}

	if lastDepth == 0 {
		return 0
	}

	// Slope = price change / depth
	priceChange := math.Abs(lastPrice - firstPrice)
	slope := priceChange / lastDepth

	return slope
}

// GetBidSlope returns bid side slope
func (qs *QuoteSlope) GetBidSlope() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.bidSlope
}

// GetAskSlope returns ask side slope
func (qs *QuoteSlope) GetAskSlope() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.askSlope
}

// GetAvgSlope returns average slope
func (qs *QuoteSlope) GetAvgSlope() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.avgSlope
}

// GetName returns indicator name
func (qs *QuoteSlope) GetName() string {
	return qs.BaseIndicator.GetName()
}

// String returns a string representation
func (qs *QuoteSlope) String() string {
	return fmt.Sprintf("QuoteSlope(levels=%d, bid=%.6f, ask=%.6f, avg=%.6f)",
		qs.levels, qs.bidSlope, qs.askSlope, qs.avgSlope)
}
