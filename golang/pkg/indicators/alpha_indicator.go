package indicators

import (
	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// AlphaIndicator calculates Jensen's Alpha (risk-adjusted excess return)
// Alpha = Portfolio_Return - [Risk_Free_Rate + Beta * (Benchmark_Return - Risk_Free_Rate)]
type AlphaIndicator struct {
	*BaseIndicator
	period           int
	portfolioReturns []float64
	benchmarkReturns []float64
	prevPortfolio    float64
	prevBenchmark    float64
	riskFreeRate     float64 // annualized risk-free rate
	lastValue        float64
	benchmarkSymbol  string
	priceCache       map[string]float64
}

// NewAlphaIndicator creates a new Alpha indicator
// benchmarkSymbol: symbol of the benchmark (e.g., index)
// riskFreeRate: annualized risk-free rate (e.g., 0.03 for 3%)
func NewAlphaIndicator(period int, benchmarkSymbol string, riskFreeRate float64, maxHistory int) *AlphaIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &AlphaIndicator{
		BaseIndicator:    NewBaseIndicator("Alpha", maxHistory),
		period:           period,
		portfolioReturns: make([]float64, 0, period),
		benchmarkReturns: make([]float64, 0, period),
		riskFreeRate:     riskFreeRate,
		benchmarkSymbol:  benchmarkSymbol,
		priceCache:       make(map[string]float64),
	}
}

// NewAlphaIndicatorFromConfig creates Alpha from configuration
func NewAlphaIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	benchmarkSymbol := ""
	riskFreeRate := 0.0
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["benchmark_symbol"]; ok {
		if s, ok := v.(string); ok {
			benchmarkSymbol = s
		}
	}

	if v, ok := config["risk_free_rate"]; ok {
		if r, ok := v.(float64); ok {
			riskFreeRate = r
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewAlphaIndicator(period, benchmarkSymbol, riskFreeRate, maxHistory), nil
}

// Update calculates Jensen's Alpha
func (a *AlphaIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Cache the price
	a.priceCache[md.Symbol] = price

	// Determine if this is portfolio or benchmark
	isPortfolio := true
	if a.benchmarkSymbol != "" && md.Symbol == a.benchmarkSymbol {
		isPortfolio = false
	}

	if isPortfolio {
		// Calculate portfolio return
		if a.prevPortfolio > 0 {
			ret := (price - a.prevPortfolio) / a.prevPortfolio
			a.portfolioReturns = append(a.portfolioReturns, ret)
			if len(a.portfolioReturns) > a.period {
				a.portfolioReturns = a.portfolioReturns[1:]
			}
		}
		a.prevPortfolio = price
	} else {
		// Calculate benchmark return
		if a.prevBenchmark > 0 {
			ret := (price - a.prevBenchmark) / a.prevBenchmark
			a.benchmarkReturns = append(a.benchmarkReturns, ret)
			if len(a.benchmarkReturns) > a.period {
				a.benchmarkReturns = a.benchmarkReturns[1:]
			}
		}
		a.prevBenchmark = price
	}

	// Need enough data for both series
	if len(a.portfolioReturns) < a.period || len(a.benchmarkReturns) < a.period {
		return
	}

	// Calculate Alpha
	a.calculateAlpha()
}

// UpdateWithReturns updates with pre-calculated returns (useful for portfolio-level calculation)
func (a *AlphaIndicator) UpdateWithReturns(portfolioReturn, benchmarkReturn float64) {
	a.portfolioReturns = append(a.portfolioReturns, portfolioReturn)
	if len(a.portfolioReturns) > a.period {
		a.portfolioReturns = a.portfolioReturns[1:]
	}

	a.benchmarkReturns = append(a.benchmarkReturns, benchmarkReturn)
	if len(a.benchmarkReturns) > a.period {
		a.benchmarkReturns = a.benchmarkReturns[1:]
	}

	if len(a.portfolioReturns) >= a.period && len(a.benchmarkReturns) >= a.period {
		a.calculateAlpha()
	}
}

func (a *AlphaIndicator) calculateAlpha() {
	// Calculate mean returns
	meanPortfolio := stats.Mean(a.portfolioReturns)
	meanBenchmark := stats.Mean(a.benchmarkReturns)

	// Calculate beta
	beta := stats.Beta(a.portfolioReturns, a.benchmarkReturns)

	// Daily risk-free rate (assuming 252 trading days)
	dailyRF := a.riskFreeRate / 252.0

	// Jensen's Alpha = Rp - [Rf + β(Rm - Rf)]
	// where Rp = portfolio return, Rm = benchmark return, Rf = risk-free rate
	expectedReturn := dailyRF + beta*(meanBenchmark-dailyRF)
	alpha := meanPortfolio - expectedReturn

	// Annualize (multiply by 252 for daily data)
	alpha *= 252.0

	a.lastValue = alpha
	a.AddValue(alpha)
}

// GetValue returns the current Alpha value (annualized)
func (a *AlphaIndicator) GetValue() float64 {
	return a.lastValue
}

// Reset resets the indicator
func (a *AlphaIndicator) Reset() {
	a.BaseIndicator.Reset()
	a.portfolioReturns = a.portfolioReturns[:0]
	a.benchmarkReturns = a.benchmarkReturns[:0]
	a.prevPortfolio = 0
	a.prevBenchmark = 0
	a.priceCache = make(map[string]float64)
	a.lastValue = 0
}

// IsReady returns true if we have enough data
func (a *AlphaIndicator) IsReady() bool {
	return len(a.portfolioReturns) >= a.period && len(a.benchmarkReturns) >= a.period
}
