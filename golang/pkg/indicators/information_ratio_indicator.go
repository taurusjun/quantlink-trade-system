package indicators

import (
	"math"

	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// InformationRatioIndicator calculates the Information Ratio
// IR = Active Return / Tracking Error
// where Active Return = Portfolio Return - Benchmark Return
// and Tracking Error = StdDev(Active Returns)
type InformationRatioIndicator struct {
	*BaseIndicator
	period           int
	portfolioReturns []float64
	benchmarkReturns []float64
	activeReturns    []float64
	prevPortfolio    float64
	prevBenchmark    float64
	lastValue        float64
	benchmarkSymbol  string
	priceCache       map[string]float64
	annualize        bool
}

// NewInformationRatioIndicator creates a new Information Ratio indicator
func NewInformationRatioIndicator(period int, benchmarkSymbol string, annualize bool, maxHistory int) *InformationRatioIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &InformationRatioIndicator{
		BaseIndicator:    NewBaseIndicator("InformationRatio", maxHistory),
		period:           period,
		portfolioReturns: make([]float64, 0, period),
		benchmarkReturns: make([]float64, 0, period),
		activeReturns:    make([]float64, 0, period),
		benchmarkSymbol:  benchmarkSymbol,
		priceCache:       make(map[string]float64),
		annualize:        annualize,
	}
}

// NewInformationRatioIndicatorFromConfig creates InformationRatio from configuration
func NewInformationRatioIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	benchmarkSymbol := ""
	annualize := true
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

	if v, ok := config["annualize"]; ok {
		if a, ok := v.(bool); ok {
			annualize = a
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewInformationRatioIndicator(period, benchmarkSymbol, annualize, maxHistory), nil
}

// Update calculates the Information Ratio
func (ir *InformationRatioIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	ir.priceCache[md.Symbol] = price

	// Determine if this is portfolio or benchmark
	isPortfolio := true
	if ir.benchmarkSymbol != "" && md.Symbol == ir.benchmarkSymbol {
		isPortfolio = false
	}

	if isPortfolio {
		if ir.prevPortfolio > 0 {
			ret := (price - ir.prevPortfolio) / ir.prevPortfolio
			ir.portfolioReturns = append(ir.portfolioReturns, ret)
			if len(ir.portfolioReturns) > ir.period {
				ir.portfolioReturns = ir.portfolioReturns[1:]
			}
		}
		ir.prevPortfolio = price
	} else {
		if ir.prevBenchmark > 0 {
			ret := (price - ir.prevBenchmark) / ir.prevBenchmark
			ir.benchmarkReturns = append(ir.benchmarkReturns, ret)
			if len(ir.benchmarkReturns) > ir.period {
				ir.benchmarkReturns = ir.benchmarkReturns[1:]
			}
		}
		ir.prevBenchmark = price
	}

	// Calculate active returns when we have both
	if len(ir.portfolioReturns) > 0 && len(ir.benchmarkReturns) > 0 {
		minLen := len(ir.portfolioReturns)
		if len(ir.benchmarkReturns) < minLen {
			minLen = len(ir.benchmarkReturns)
		}

		// Recalculate active returns from the available data
		ir.activeReturns = make([]float64, minLen)
		for i := 0; i < minLen; i++ {
			portIdx := len(ir.portfolioReturns) - minLen + i
			benchIdx := len(ir.benchmarkReturns) - minLen + i
			ir.activeReturns[i] = ir.portfolioReturns[portIdx] - ir.benchmarkReturns[benchIdx]
		}
	}

	if len(ir.activeReturns) < ir.period {
		return
	}

	ir.calculateIR()
}

// UpdateWithReturns updates with pre-calculated returns
func (ir *InformationRatioIndicator) UpdateWithReturns(portfolioReturn, benchmarkReturn float64) {
	ir.portfolioReturns = append(ir.portfolioReturns, portfolioReturn)
	if len(ir.portfolioReturns) > ir.period {
		ir.portfolioReturns = ir.portfolioReturns[1:]
	}

	ir.benchmarkReturns = append(ir.benchmarkReturns, benchmarkReturn)
	if len(ir.benchmarkReturns) > ir.period {
		ir.benchmarkReturns = ir.benchmarkReturns[1:]
	}

	activeReturn := portfolioReturn - benchmarkReturn
	ir.activeReturns = append(ir.activeReturns, activeReturn)
	if len(ir.activeReturns) > ir.period {
		ir.activeReturns = ir.activeReturns[1:]
	}

	if len(ir.activeReturns) >= ir.period {
		ir.calculateIR()
	}
}

func (ir *InformationRatioIndicator) calculateIR() {
	// Calculate mean active return
	meanActive := stats.Mean(ir.activeReturns)

	// Calculate tracking error (std dev of active returns)
	trackingError := stats.StdDev(ir.activeReturns)

	if trackingError == 0 {
		ir.lastValue = 0
		ir.AddValue(0)
		return
	}

	// Information Ratio = Mean Active Return / Tracking Error
	infoRatio := meanActive / trackingError

	// Annualize if requested (multiply by sqrt(252))
	if ir.annualize {
		infoRatio *= math.Sqrt(252.0)
	}

	ir.lastValue = infoRatio
	ir.AddValue(infoRatio)
}

// GetValue returns the current Information Ratio
func (ir *InformationRatioIndicator) GetValue() float64 {
	return ir.lastValue
}

// GetTrackingError returns the current tracking error
func (ir *InformationRatioIndicator) GetTrackingError() float64 {
	if len(ir.activeReturns) < 2 {
		return 0
	}
	te := stats.StdDev(ir.activeReturns)
	if ir.annualize {
		te *= math.Sqrt(252.0)
	}
	return te
}

// GetActiveReturn returns the mean active return
func (ir *InformationRatioIndicator) GetActiveReturn() float64 {
	if len(ir.activeReturns) == 0 {
		return 0
	}
	ar := stats.Mean(ir.activeReturns)
	if ir.annualize {
		ar *= 252.0
	}
	return ar
}

// Reset resets the indicator
func (ir *InformationRatioIndicator) Reset() {
	ir.BaseIndicator.Reset()
	ir.portfolioReturns = ir.portfolioReturns[:0]
	ir.benchmarkReturns = ir.benchmarkReturns[:0]
	ir.activeReturns = ir.activeReturns[:0]
	ir.prevPortfolio = 0
	ir.prevBenchmark = 0
	ir.priceCache = make(map[string]float64)
	ir.lastValue = 0
}

// IsReady returns true if we have enough data
func (ir *InformationRatioIndicator) IsReady() bool {
	return len(ir.activeReturns) >= ir.period
}
