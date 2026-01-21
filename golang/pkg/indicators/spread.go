package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Spread calculates the bid-ask spread
type Spread struct {
	*BaseIndicator
	absolute   bool // Absolute spread vs percentage spread
	ewmaSmooth *EWMA // Optional EWMA smoothing
}

// NewSpread creates a new Spread indicator
func NewSpread(absolute bool, maxHistory int) *Spread {
	return &Spread{
		BaseIndicator: NewBaseIndicator("Spread", maxHistory),
		absolute:      absolute,
	}
}

// NewSpreadWithSmoothing creates a Spread indicator with EWMA smoothing
func NewSpreadWithSmoothing(absolute bool, smoothingAlpha float64, maxHistory int) *Spread {
	s := NewSpread(absolute, maxHistory)
	if smoothingAlpha > 0 && smoothingAlpha <= 1 {
		s.ewmaSmooth = NewEWMA(smoothingAlpha, maxHistory)
	}
	return s
}

// NewSpreadFromConfig creates Spread from configuration
func NewSpreadFromConfig(config map[string]interface{}) (Indicator, error) {
	absolute := true
	maxHistory := 1000
	var smoothingAlpha float64

	if v, ok := config["absolute"]; ok {
		if a, ok := v.(bool); ok {
			absolute = a
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if v, ok := config["smoothing_alpha"]; ok {
		if a, ok := v.(float64); ok {
			smoothingAlpha = a
		}
	}

	if smoothingAlpha < 0 || smoothingAlpha > 1 {
		smoothingAlpha = 0
	}

	if smoothingAlpha > 0 {
		return NewSpreadWithSmoothing(absolute, smoothingAlpha, maxHistory), nil
	}
	return NewSpread(absolute, maxHistory), nil
}

// Update calculates the spread from market data
func (s *Spread) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	spread := md.AskPrice[0] - md.BidPrice[0]

	if !s.absolute {
		// Calculate percentage spread
		midPrice := GetMidPrice(md)
		if midPrice > 0 {
			spread = (spread / midPrice) * 100.0
		}
	}

	if s.ewmaSmooth != nil {
		// Use synthetic market data for smoothing
		syntheticMd := &mdpb.MarketDataUpdate{
			Symbol:    md.Symbol,
			Exchange:  md.Exchange,
			Timestamp: md.Timestamp,
			BidPrice:  []float64{spread},
			AskPrice:  []float64{spread},
		}
		s.ewmaSmooth.Update(syntheticMd)
		s.AddValue(s.ewmaSmooth.GetValue())
	} else {
		s.AddValue(spread)
	}
}

// Reset resets the indicator
func (s *Spread) Reset() {
	s.BaseIndicator.Reset()
	if s.ewmaSmooth != nil {
		s.ewmaSmooth.Reset()
	}
}

// IsReady returns true
func (s *Spread) IsReady() bool {
	return true
}

// EffectiveSpread calculates the effective spread (considering trade direction)
type EffectiveSpread struct {
	*BaseIndicator
	lastMidPrice float64
}

// NewEffectiveSpread creates a new EffectiveSpread indicator
func NewEffectiveSpread(maxHistory int) *EffectiveSpread {
	return &EffectiveSpread{
		BaseIndicator: NewBaseIndicator("EffectiveSpread", maxHistory),
	}
}

// Update calculates the effective spread
func (es *EffectiveSpread) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	// Effective spread = 2 * |trade_price - mid_price|
	if md.LastPrice > 0 && es.lastMidPrice > 0 {
		effectiveSpread := 2 * abs(md.LastPrice-es.lastMidPrice)
		es.AddValue(effectiveSpread)
	}

	es.lastMidPrice = midPrice
}

// Reset resets the indicator
func (es *EffectiveSpread) Reset() {
	es.BaseIndicator.Reset()
	es.lastMidPrice = 0
}

// IsReady returns true if we have a mid price
func (es *EffectiveSpread) IsReady() bool {
	return es.lastMidPrice > 0
}

// RealizedSpread calculates the realized spread over a time window
type RealizedSpread struct {
	*BaseIndicator
	window       int
	tradePrices  []float64
	midPrices    []float64
}

// NewRealizedSpread creates a new RealizedSpread indicator
func NewRealizedSpread(window int, maxHistory int) *RealizedSpread {
	if window <= 0 {
		window = 10
	}

	return &RealizedSpread{
		BaseIndicator: NewBaseIndicator("RealizedSpread", maxHistory),
		window:        window,
		tradePrices:   make([]float64, 0, window),
		midPrices:     make([]float64, 0, window),
	}
}

// Update calculates the realized spread
func (rs *RealizedSpread) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	if md.LastPrice > 0 {
		// Add current values
		rs.tradePrices = append(rs.tradePrices, md.LastPrice)
		rs.midPrices = append(rs.midPrices, midPrice)

		// Maintain window size
		if len(rs.tradePrices) > rs.window {
			rs.tradePrices = rs.tradePrices[1:]
			rs.midPrices = rs.midPrices[1:]
		}

		// Calculate realized spread if we have enough data
		if len(rs.tradePrices) >= 2 {
			// Average spread over the window
			var sumSpread float64
			for i := range rs.tradePrices {
				spread := 2 * abs(rs.tradePrices[i]-rs.midPrices[i])
				sumSpread += spread
			}
			avgSpread := sumSpread / float64(len(rs.tradePrices))
			rs.AddValue(avgSpread)
		}
	}
}

// Reset resets the indicator
func (rs *RealizedSpread) Reset() {
	rs.BaseIndicator.Reset()
	rs.tradePrices = rs.tradePrices[:0]
	rs.midPrices = rs.midPrices[:0]
}

// IsReady returns true if we have enough data
func (rs *RealizedSpread) IsReady() bool {
	return len(rs.tradePrices) >= 2
}

// QuotedSpread calculates the spread at different depth levels
type QuotedSpread struct {
	*BaseIndicator
	level int // Depth level (0 = best, 1 = second best, etc.)
}

// NewQuotedSpread creates a new QuotedSpread indicator
func NewQuotedSpread(level int, maxHistory int) *QuotedSpread {
	if level < 0 {
		level = 0
	}

	return &QuotedSpread{
		BaseIndicator: NewBaseIndicator(fmt.Sprintf("QuotedSpread_L%d", level), maxHistory),
		level:         level,
	}
}

// Update calculates the quoted spread at specified level
func (qs *QuotedSpread) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) <= qs.level || len(md.AskPrice) <= qs.level {
		return
	}

	spread := md.AskPrice[qs.level] - md.BidPrice[qs.level]
	qs.AddValue(spread)
}

// Reset resets the indicator
func (qs *QuotedSpread) Reset() {
	qs.BaseIndicator.Reset()
}

// IsReady returns true
func (qs *QuotedSpread) IsReady() bool {
	return true
}

// Helper function
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
