package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Volatility calculates price volatility using various methods
type Volatility struct {
	*BaseIndicator
	window      int
	returns     []float64
	lastPrice   float64
	useLogReturns bool
	annualizationFactor float64
}

// NewVolatility creates a new Volatility indicator
func NewVolatility(window int, useLogReturns bool, maxHistory int) *Volatility {
	if window <= 0 {
		window = 20 // Default 20-period volatility
	}

	return &Volatility{
		BaseIndicator: NewBaseIndicator("Volatility", maxHistory),
		window:        window,
		returns:       make([]float64, 0, window),
		useLogReturns: useLogReturns,
		annualizationFactor: math.Sqrt(252), // Assuming 252 trading days
	}
}

// NewVolatilityFromConfig creates Volatility from configuration
func NewVolatilityFromConfig(config map[string]interface{}) (Indicator, error) {
	window := 20
	useLogReturns := true
	maxHistory := 1000

	if v, ok := config["window"]; ok {
		if w, ok := v.(float64); ok {
			window = int(w)
		}
	}

	if v, ok := config["use_log_returns"]; ok {
		if l, ok := v.(bool); ok {
			useLogReturns = l
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if window <= 0 {
		return nil, fmt.Errorf("%w: window must be positive", ErrInvalidParameter)
	}

	return NewVolatility(window, useLogReturns, maxHistory), nil
}

// Update calculates volatility from market data
func (v *Volatility) Update(md *mdpb.MarketDataUpdate) {
	price := md.LastPrice
	if price == 0 {
		price = GetMidPrice(md)
	}

	if price == 0 {
		return
	}

	// Calculate return if we have a previous price
	if v.lastPrice > 0 {
		var ret float64
		if v.useLogReturns {
			ret = math.Log(price / v.lastPrice)
		} else {
			ret = (price - v.lastPrice) / v.lastPrice
		}

		// Add return to window
		v.returns = append(v.returns, ret)
		if len(v.returns) > v.window {
			v.returns = v.returns[1:]
		}

		// Calculate volatility if we have enough data
		if len(v.returns) >= 2 {
			volatility := v.calculateStdDev()
			v.AddValue(volatility)
		}
	}

	v.lastPrice = price
}

// calculateStdDev calculates standard deviation of returns
func (v *Volatility) calculateStdDev() float64 {
	if len(v.returns) == 0 {
		return 0.0
	}

	// Calculate mean
	var sum float64
	for _, ret := range v.returns {
		sum += ret
	}
	mean := sum / float64(len(v.returns))

	// Calculate variance
	var variance float64
	for _, ret := range v.returns {
		diff := ret - mean
		variance += diff * diff
	}
	variance /= float64(len(v.returns))

	// Return standard deviation
	return math.Sqrt(variance)
}

// GetAnnualizedValue returns annualized volatility
func (v *Volatility) GetAnnualizedValue() float64 {
	return v.GetValue() * v.annualizationFactor
}

// Reset resets the indicator
func (v *Volatility) Reset() {
	v.BaseIndicator.Reset()
	v.returns = v.returns[:0]
	v.lastPrice = 0
}

// IsReady returns true if we have enough data
func (v *Volatility) IsReady() bool {
	return len(v.returns) >= 2
}

// EWMAVolatility calculates EWMA-based volatility (like EWMA of squared returns)
type EWMAVolatility struct {
	*BaseIndicator
	lambda       float64
	variance     float64
	lastPrice    float64
	useLogReturns bool
	isInit       bool
}

// NewEWMAVolatility creates a new EWMA volatility indicator
func NewEWMAVolatility(lambda float64, useLogReturns bool, maxHistory int) *EWMAVolatility {
	if lambda <= 0 || lambda >= 1 {
		lambda = 0.94 // Common RiskMetrics value
	}

	return &EWMAVolatility{
		BaseIndicator: NewBaseIndicator("EWMAVolatility", maxHistory),
		lambda:        lambda,
		useLogReturns: useLogReturns,
	}
}

// Update updates EWMA volatility
func (ev *EWMAVolatility) Update(md *mdpb.MarketDataUpdate) {
	price := md.LastPrice
	if price == 0 {
		price = GetMidPrice(md)
	}

	if price == 0 {
		return
	}

	if ev.lastPrice > 0 {
		// Calculate return
		var ret float64
		if ev.useLogReturns {
			ret = math.Log(price / ev.lastPrice)
		} else {
			ret = (price - ev.lastPrice) / ev.lastPrice
		}

		// Update variance using EWMA
		squaredReturn := ret * ret
		if !ev.isInit {
			ev.variance = squaredReturn
			ev.isInit = true
		} else {
			ev.variance = ev.lambda*ev.variance + (1-ev.lambda)*squaredReturn
		}

		// Store volatility (square root of variance)
		volatility := math.Sqrt(ev.variance)
		ev.AddValue(volatility)
	}

	ev.lastPrice = price
}

// Reset resets the indicator
func (ev *EWMAVolatility) Reset() {
	ev.BaseIndicator.Reset()
	ev.variance = 0
	ev.lastPrice = 0
	ev.isInit = false
}

// IsReady returns true if initialized
func (ev *EWMAVolatility) IsReady() bool {
	return ev.isInit
}

// ParkinsonVolatility calculates volatility using high-low range
type ParkinsonVolatility struct {
	*BaseIndicator
	window    int
	hlRatios  []float64
}

// NewParkinsonVolatility creates Parkinson volatility estimator
func NewParkinsonVolatility(window int, maxHistory int) *ParkinsonVolatility {
	if window <= 0 {
		window = 20
	}

	return &ParkinsonVolatility{
		BaseIndicator: NewBaseIndicator("ParkinsonVolatility", maxHistory),
		window:        window,
		hlRatios:      make([]float64, 0, window),
	}
}

// Update updates Parkinson volatility
func (pv *ParkinsonVolatility) Update(md *mdpb.MarketDataUpdate) {
	// Need both high and low prices
	if md.HighPrice == 0 || md.LowPrice == 0 {
		return
	}

	// Calculate log(High/Low)^2
	logHLRatio := math.Log(md.HighPrice / md.LowPrice)
	squaredLogRatio := logHLRatio * logHLRatio

	pv.hlRatios = append(pv.hlRatios, squaredLogRatio)
	if len(pv.hlRatios) > pv.window {
		pv.hlRatios = pv.hlRatios[1:]
	}

	if len(pv.hlRatios) >= 2 {
		// Parkinson estimator: sqrt(1/(4*ln(2)) * mean(log(H/L)^2))
		var sum float64
		for _, ratio := range pv.hlRatios {
			sum += ratio
		}
		mean := sum / float64(len(pv.hlRatios))

		// Parkinson constant
		constant := 1.0 / (4.0 * math.Log(2))
		volatility := math.Sqrt(constant * mean)

		pv.AddValue(volatility)
	}
}

// Reset resets the indicator
func (pv *ParkinsonVolatility) Reset() {
	pv.BaseIndicator.Reset()
	pv.hlRatios = pv.hlRatios[:0]
}

// IsReady returns true if we have enough data
func (pv *ParkinsonVolatility) IsReady() bool {
	return len(pv.hlRatios) >= 2
}

// GarmanKlassVolatility calculates Garman-Klass volatility estimator
type GarmanKlassVolatility struct {
	*BaseIndicator
	window    int
	values    []garmanKlassValues
}

type garmanKlassValues struct {
	open  float64
	high  float64
	low   float64
	close float64
}

// NewGarmanKlassVolatility creates Garman-Klass volatility estimator
func NewGarmanKlassVolatility(window int, maxHistory int) *GarmanKlassVolatility {
	if window <= 0 {
		window = 20
	}

	return &GarmanKlassVolatility{
		BaseIndicator: NewBaseIndicator("GarmanKlassVolatility", maxHistory),
		window:        window,
		values:        make([]garmanKlassValues, 0, window),
	}
}

// Update updates Garman-Klass volatility
func (gk *GarmanKlassVolatility) Update(md *mdpb.MarketDataUpdate) {
	// Need OHLC data
	if md.OpenPrice == 0 || md.HighPrice == 0 || md.LowPrice == 0 {
		return
	}

	close := md.LastPrice
	if close == 0 {
		close = GetMidPrice(md)
	}

	gk.values = append(gk.values, garmanKlassValues{
		open:  md.OpenPrice,
		high:  md.HighPrice,
		low:   md.LowPrice,
		close: close,
	})

	if len(gk.values) > gk.window {
		gk.values = gk.values[1:]
	}

	if len(gk.values) >= 2 {
		var sum float64
		for _, v := range gk.values {
			// Garman-Klass formula:
			// 0.5 * (log(H/L))^2 - (2*log(2)-1) * (log(C/O))^2
			logHL := math.Log(v.high / v.low)
			logCO := math.Log(v.close / v.open)

			term1 := 0.5 * logHL * logHL
			term2 := (2*math.Log(2) - 1) * logCO * logCO

			sum += term1 - term2
		}

		mean := sum / float64(len(gk.values))
		volatility := math.Sqrt(mean)

		gk.AddValue(volatility)
	}
}

// Reset resets the indicator
func (gk *GarmanKlassVolatility) Reset() {
	gk.BaseIndicator.Reset()
	gk.values = gk.values[:0]
}

// IsReady returns true if we have enough data
func (gk *GarmanKlassVolatility) IsReady() bool {
	return len(gk.values) >= 2
}
