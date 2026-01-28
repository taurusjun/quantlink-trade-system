package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// HMA (Hull Moving Average) provides a fast and smooth moving average
// that reduces lag while maintaining smoothness.
// HMA(n) = WMA(2*WMA(n/2) - WMA(n), sqrt(n))
type HMA struct {
	*BaseIndicator
	period    int
	wma1      *WMA // WMA(n/2)
	wma2      *WMA // WMA(n)
	wmaFinal  *WMA // WMA(sqrt(n))
	tempData  []float64
	lastValue float64
}

// NewHMA creates a new Hull Moving Average indicator
func NewHMA(period int, maxHistory int) *HMA {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	halfPeriod := period / 2
	if halfPeriod < 1 {
		halfPeriod = 1
	}

	// Calculate sqrt(period) for final WMA
	sqrtPeriod := int(float64(period) * 0.7071) // approximation of sqrt
	if sqrtPeriod < 1 {
		sqrtPeriod = 1
	}

	return &HMA{
		BaseIndicator: NewBaseIndicator("HMA", maxHistory),
		period:        period,
		wma1:          NewWMA(halfPeriod, maxHistory),
		wma2:          NewWMA(period, maxHistory),
		wmaFinal:      NewWMA(sqrtPeriod, maxHistory),
		tempData:      make([]float64, 0, sqrtPeriod),
	}
}

// NewHMAFromConfig creates HMA from configuration
func NewHMAFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewHMA(period, maxHistory), nil
}

// Update calculates the Hull Moving Average
func (h *HMA) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Update both WMAs with the current price
	h.wma1.Update(md)
	h.wma2.Update(md)

	// Wait until both WMAs are ready
	if !h.wma1.IsReady() || !h.wma2.IsReady() {
		return
	}

	// Calculate: 2*WMA(n/2) - WMA(n)
	rawHMA := 2*h.wma1.GetValue() - h.wma2.GetValue()

	// Feed this value to the final WMA
	// Create a dummy market data update with the rawHMA as price
	dummyMD := &mdpb.MarketDataUpdate{
		BidPrice: []float64{rawHMA},
		AskPrice: []float64{rawHMA},
	}
	h.wmaFinal.Update(dummyMD)

	if h.wmaFinal.IsReady() {
		h.lastValue = h.wmaFinal.GetValue()
		h.AddValue(h.lastValue)
	}
}

// GetValue returns the current HMA value
func (h *HMA) GetValue() float64 {
	return h.lastValue
}

// GetPeriod returns the period
func (h *HMA) GetPeriod() int {
	return h.period
}

// Reset resets the indicator
func (h *HMA) Reset() {
	h.BaseIndicator.Reset()
	h.wma1.Reset()
	h.wma2.Reset()
	h.wmaFinal.Reset()
	h.tempData = h.tempData[:0]
	h.lastValue = 0
}

// IsReady returns true if the indicator has sufficient data
func (h *HMA) IsReady() bool {
	return h.wmaFinal.IsReady()
}
