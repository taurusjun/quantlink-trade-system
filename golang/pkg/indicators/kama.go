package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// KAMA (Kaufman Adaptive Moving Average) is an adaptive moving average that
// adjusts its smoothing factor based on market volatility and efficiency.
//
// Developed by Perry Kaufman, KAMA reduces lag in trending markets and
// increases lag in sideways markets to filter out noise.
//
// Formula:
// 1. Efficiency Ratio (ER) = |Change| / Volatility
//    - Change = |Close - Close[period]|
//    - Volatility = Sum(|Close - Close[1]|, period)
// 2. Smoothing Constant (SC) = [ER × (fastest - slowest) + slowest]²
//    - fastest = 2/(2+1) = 0.6667 (for fast EMA)
//    - slowest = 2/(30+1) = 0.0645 (for slow EMA)
// 3. KAMA = KAMA_prev + SC × (Price - KAMA_prev)
//
// Properties:
// - Adaptive to market conditions
// - Fast in trending markets (high ER)
// - Slow in ranging markets (low ER)
// - Reduces whipsaws
// - Less lag than traditional MAs
type KAMA struct {
	*BaseIndicator
	period       int
	fastPeriod   int
	slowPeriod   int
	prices       []float64
	kama         float64
	fastSC       float64 // Fastest smoothing constant
	slowSC       float64 // Slowest smoothing constant
	initialized  bool
}

// NewKAMA creates a new KAMA indicator
func NewKAMA(period int, fastPeriod int, slowPeriod int, maxHistory int) *KAMA {
	if period <= 0 {
		period = 10
	}
	if fastPeriod <= 0 {
		fastPeriod = 2
	}
	if slowPeriod <= 0 {
		slowPeriod = 30
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Calculate smoothing constants
	fastSC := 2.0 / float64(fastPeriod+1)
	slowSC := 2.0 / float64(slowPeriod+1)

	return &KAMA{
		BaseIndicator: NewBaseIndicator("KAMA", maxHistory),
		period:        period,
		fastPeriod:    fastPeriod,
		slowPeriod:    slowPeriod,
		prices:        make([]float64, 0, period+1),
		fastSC:        fastSC,
		slowSC:        slowSC,
	}
}

// NewKAMAFromConfig creates KAMA from configuration
func NewKAMAFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 10
	fastPeriod := 2
	slowPeriod := 30
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["fast_period"]; ok {
		if p, ok := v.(float64); ok {
			fastPeriod = int(p)
		}
	}

	if v, ok := config["slow_period"]; ok {
		if p, ok := v.(float64); ok {
			slowPeriod = int(p)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewKAMA(period, fastPeriod, slowPeriod, maxHistory), nil
}

// Update updates the indicator with new market data
func (k *KAMA) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Store price
	k.prices = append(k.prices, price)

	// Keep only period+1 values
	if len(k.prices) > k.period+1 {
		k.prices = k.prices[1:]
	}

	// Initialize KAMA with first price
	if !k.initialized {
		if len(k.prices) >= k.period+1 {
			k.kama = k.prices[0]
			k.initialized = true
		} else {
			return
		}
	}

	// Need at least period+1 values
	if len(k.prices) < k.period+1 {
		return
	}

	// Calculate Efficiency Ratio (ER)
	change := math.Abs(k.prices[len(k.prices)-1] - k.prices[0])

	// Calculate Volatility (sum of absolute price changes)
	volatility := 0.0
	for i := 1; i < len(k.prices); i++ {
		volatility += math.Abs(k.prices[i] - k.prices[i-1])
	}

	// Calculate ER (avoid division by zero)
	var er float64
	if volatility > 0 {
		er = change / volatility
	} else {
		er = 0
	}

	// Calculate Smoothing Constant (SC)
	// SC = [ER × (fastest - slowest) + slowest]²
	sc := er * (k.fastSC - k.slowSC) + k.slowSC
	sc = sc * sc // Square it

	// Update KAMA
	// KAMA = KAMA_prev + SC × (Price - KAMA_prev)
	k.kama = k.kama + sc*(price-k.kama)

	k.AddValue(k.kama)
}

// GetValue returns the current KAMA value
func (k *KAMA) GetValue() float64 {
	return k.kama
}

// Reset resets the indicator
func (k *KAMA) Reset() {
	k.BaseIndicator.Reset()
	k.prices = k.prices[:0]
	k.kama = 0
	k.initialized = false
}

// IsReady returns true if the indicator has enough data
func (k *KAMA) IsReady() bool {
	return k.initialized && len(k.prices) >= k.period+1
}

// GetPeriod returns the period
func (k *KAMA) GetPeriod() int {
	return k.period
}

// GetFastPeriod returns the fast period
func (k *KAMA) GetFastPeriod() int {
	return k.fastPeriod
}

// GetSlowPeriod returns the slow period
func (k *KAMA) GetSlowPeriod() int {
	return k.slowPeriod
}

// GetEfficiencyRatio returns the current efficiency ratio
// Useful for understanding market conditions
func (k *KAMA) GetEfficiencyRatio() float64 {
	if len(k.prices) < k.period+1 {
		return 0
	}

	change := math.Abs(k.prices[len(k.prices)-1] - k.prices[0])

	volatility := 0.0
	for i := 1; i < len(k.prices); i++ {
		volatility += math.Abs(k.prices[i] - k.prices[i-1])
	}

	if volatility > 0 {
		return change / volatility
	}
	return 0
}

// IsTrendingMarket returns true if ER is high (> 0.3)
// High ER indicates trending market
func (k *KAMA) IsTrendingMarket() bool {
	return k.GetEfficiencyRatio() > 0.3
}

// IsRangingMarket returns true if ER is low (< 0.2)
// Low ER indicates ranging/choppy market
func (k *KAMA) IsRangingMarket() bool {
	return k.GetEfficiencyRatio() < 0.2
}

// GetTrend returns the trend direction
// Returns 1 for uptrend, -1 for downtrend, 0 for neutral
func (k *KAMA) GetTrend() int {
	if len(k.values) < 2 {
		return 0
	}

	current := k.values[len(k.values)-1]
	previous := k.values[len(k.values)-2]

	if current > previous {
		return 1
	} else if current < previous {
		return -1
	}

	return 0
}
