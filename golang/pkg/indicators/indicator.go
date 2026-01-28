// Package indicators provides technical indicators for trading strategies
package indicators

import (
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Indicator is the base interface for all technical indicators
type Indicator interface {
	// Update updates the indicator with new market data
	Update(md *mdpb.MarketDataUpdate)

	// GetValue returns the current indicator value
	GetValue() float64

	// GetValues returns a time series of indicator values
	GetValues() []float64

	// Reset resets the indicator to initial state
	Reset()

	// GetName returns the indicator name
	GetName() string

	// IsReady returns true if the indicator has sufficient data
	IsReady() bool
}

// BaseIndicator provides common functionality for indicators
type BaseIndicator struct {
	name       string
	values     []float64
	maxHistory int
	mu         sync.RWMutex
	initialized bool
}

// NewBaseIndicator creates a new base indicator
func NewBaseIndicator(name string, maxHistory int) *BaseIndicator {
	return &BaseIndicator{
		name:       name,
		values:     make([]float64, 0, maxHistory),
		maxHistory: maxHistory,
	}
}

// GetName returns the indicator name
func (b *BaseIndicator) GetName() string {
	return b.name
}

// GetValue returns the most recent value
func (b *BaseIndicator) GetValue() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.values) == 0 {
		return 0.0
	}
	return b.values[len(b.values)-1]
}

// GetValues returns all historical values
func (b *BaseIndicator) GetValues() []float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]float64, len(b.values))
	copy(result, b.values)
	return result
}

// AddValue adds a new value to the time series
func (b *BaseIndicator) AddValue(value float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.values = append(b.values, value)
	if len(b.values) > b.maxHistory {
		b.values = b.values[1:]
	}
	b.initialized = true
}

// Reset clears all values
func (b *BaseIndicator) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.values = b.values[:0]
	b.initialized = false
}

// IsReady returns true if initialized
func (b *BaseIndicator) IsReady() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// GetMidPrice calculates the mid price from market data
func GetMidPrice(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return 0.0
	}
	return (md.BidPrice[0] + md.AskPrice[0]) / 2.0
}

// GetSpread calculates the bid-ask spread
func GetSpread(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return 0.0
	}
	return md.AskPrice[0] - md.BidPrice[0]
}

// GetWeightedMidPrice calculates volume-weighted mid price
func GetWeightedMidPrice(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 ||
		len(md.BidQty) == 0 || len(md.AskQty) == 0 {
		return 0.0
	}

	bidVol := float64(md.BidQty[0])
	askVol := float64(md.AskQty[0])
	totalVol := bidVol + askVol

	if totalVol == 0 {
		return GetMidPrice(md)
	}

	return (md.BidPrice[0]*askVol + md.AskPrice[0]*bidVol) / totalVol
}

// IndicatorLibrary manages a collection of indicators
type IndicatorLibrary struct {
	indicators map[string]Indicator
	factories  map[string]IndicatorFactory
	mu         sync.RWMutex
}

// IndicatorFactory creates indicators with configuration
type IndicatorFactory func(config map[string]interface{}) (Indicator, error)

// IndicatorConfig holds indicator configuration
type IndicatorConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewIndicatorLibrary creates a new indicator library
func NewIndicatorLibrary() *IndicatorLibrary {
	lib := &IndicatorLibrary{
		indicators: make(map[string]Indicator),
		factories:  make(map[string]IndicatorFactory),
	}

	// Register built-in indicators
	lib.RegisterFactory("ewma", NewEWMAFromConfig)
	lib.RegisterFactory("order_imbalance", NewOrderImbalanceFromConfig)
	lib.RegisterFactory("vwap", NewVWAPFromConfig)
	lib.RegisterFactory("spread", NewSpreadFromConfig)
	lib.RegisterFactory("volatility", NewVolatilityFromConfig)
	lib.RegisterFactory("rsi", NewRSIFromConfig)
	lib.RegisterFactory("macd", NewMACDFromConfig)
	lib.RegisterFactory("sma", NewSMAFromConfig)
	lib.RegisterFactory("bollinger_bands", NewBollingerBandsFromConfig)
	lib.RegisterFactory("atr", NewATRFromConfig)

	// Register orderbook indicators - Depth Indicators (Group 1)
	lib.RegisterFactory("book_depth", NewBookDepthFromConfig)
	lib.RegisterFactory("cumulative_volume", NewCumulativeVolumeFromConfig)
	lib.RegisterFactory("depth_imbalance", NewDepthImbalanceFromConfig)
	lib.RegisterFactory("volume_at_price", NewVolumeAtPriceFromConfig)
	lib.RegisterFactory("book_pressure", NewBookPressureFromConfig)

	// Liquidity Indicators (Group 2)
	lib.RegisterFactory("liquidity_score", NewLiquidityScoreFromConfig)
	lib.RegisterFactory("market_depth", NewMarketDepthFromConfig)
	lib.RegisterFactory("quote_slope", NewQuoteSlopeFromConfig)
	lib.RegisterFactory("depth_to_spread", NewDepthToSpreadFromConfig)
	lib.RegisterFactory("resilience_score", NewResilienceScoreFromConfig)

	// Order Flow Indicators (Group 3)
	lib.RegisterFactory("order_flow_imbalance", NewOrderFlowImbalanceFromConfig)
	lib.RegisterFactory("trade_intensity", NewTradeIntensityFromConfig)
	lib.RegisterFactory("buy_sell_pressure", NewBuySellPressureFromConfig)
	lib.RegisterFactory("net_order_flow", NewNetOrderFlowFromConfig)
	lib.RegisterFactory("aggressive_trade", NewAggressiveTradeFromConfig)

	// Microstructure Indicators (Group 4)
	lib.RegisterFactory("tick_rule", NewTickRuleFromConfig)
	lib.RegisterFactory("quote_stability", NewQuoteStabilityFromConfig)
	lib.RegisterFactory("spread_volatility", NewSpreadVolatilityFromConfig)
	lib.RegisterFactory("quote_update_frequency", NewQuoteUpdateFrequencyFromConfig)
	lib.RegisterFactory("order_arrival_rate", NewOrderArrivalRateFromConfig)

	// Core Orderbook Indicators (P0 Task #3)
	lib.RegisterFactory("avg_book_size", NewAvgBookSizeFromConfig)
	lib.RegisterFactory("avg_bid_qty", NewAvgBidQtyFromConfig)
	lib.RegisterFactory("avg_ask_qty", NewAvgAskQtyFromConfig)
	lib.RegisterFactory("avg_spread", NewAvgSpreadFromConfig)
	lib.RegisterFactory("bbd", NewBBDFromConfig)
	lib.RegisterFactory("bad", NewBADFromConfig)
	lib.RegisterFactory("msw_price", NewMSWPriceFromConfig)
	lib.RegisterFactory("mow_price", NewMOWPriceFromConfig)

	// Legacy orderbook indicators (kept for compatibility)
	lib.RegisterFactory("mid_price", NewMidPriceFromConfig)
	lib.RegisterFactory("weighted_mid_price", NewWeightedMidPriceFromConfig)
	lib.RegisterFactory("orderbook_volume", NewOrderBookVolumeFromConfig)
	lib.RegisterFactory("price_impact", NewPriceImpactFromConfig)
	lib.RegisterFactory("liquidity_ratio", NewLiquidityRatioFromConfig)
	lib.RegisterFactory("bid_ask_ratio", NewBidAskRatioFromConfig)

	// Register technical indicators
	lib.RegisterFactory("wma", NewWMAFromConfig)
	lib.RegisterFactory("momentum", NewMomentumFromConfig)
	lib.RegisterFactory("roc", NewROCFromConfig)
	lib.RegisterFactory("stddev", NewStdDevFromConfig)

	// Core Technical Indicators (P0 Task #4)
	lib.RegisterFactory("hma", NewHMAFromConfig)
	lib.RegisterFactory("return", NewReturnFromConfig)
	lib.RegisterFactory("correlation_indicator", NewCorrelationIndicatorFromConfig)
	lib.RegisterFactory("covariance_indicator", NewCovarianceIndicatorFromConfig)
	lib.RegisterFactory("beta_indicator", NewBetaIndicatorFromConfig)
	lib.RegisterFactory("sharpe_ratio", NewSharpeRatioIndicatorFromConfig)
	lib.RegisterFactory("max_drawdown", NewMaxDrawdownIndicatorFromConfig)
	lib.RegisterFactory("alpha", NewAlphaIndicatorFromConfig)
	lib.RegisterFactory("information_ratio", NewInformationRatioIndicatorFromConfig)
	lib.RegisterFactory("cointegration", NewCointegrationIndicatorFromConfig)

	// P1-2: Additional Technical Indicators (Batch 1 - Moving Averages)
	lib.RegisterFactory("ema", NewEMAFromConfig)
	lib.RegisterFactory("tema", NewTEMAFromConfig)

	// P1-2: Additional Technical Indicators (Batch 2 - Oscillators)
	lib.RegisterFactory("williams_r", NewWilliamsRFromConfig)
	lib.RegisterFactory("stochastic", NewStochasticFromConfig)
	lib.RegisterFactory("cci", NewCCIFromConfig)
	lib.RegisterFactory("cmo", NewCMOFromConfig)
	lib.RegisterFactory("adx", NewADXFromConfig)

	// P1-2: Additional Technical Indicators (Batch 3 - Trend Following)
	lib.RegisterFactory("trix", NewTRIXFromConfig)
	lib.RegisterFactory("aroon", NewAroonFromConfig)
	lib.RegisterFactory("supertrend", NewSupertrendFromConfig)
	lib.RegisterFactory("dmi", NewDMIFromConfig)
	lib.RegisterFactory("psar", NewParabolicSARFromConfig)

	// P1-2: Additional Technical Indicators (Batch 4 - Volume)
	lib.RegisterFactory("obv", NewOBVFromConfig)
	lib.RegisterFactory("mfi", NewMFIFromConfig)
	lib.RegisterFactory("pvt", NewPVTFromConfig)

	// P1-2: Additional Technical Indicators (Batch 5 - Channels)
	lib.RegisterFactory("donchian", NewDonchianChannelsFromConfig)

	return lib
}

// RegisterFactory registers an indicator factory
func (lib *IndicatorLibrary) RegisterFactory(indicatorType string, factory IndicatorFactory) {
	lib.mu.Lock()
	defer lib.mu.Unlock()
	lib.factories[indicatorType] = factory
}

// Create creates an indicator instance
func (lib *IndicatorLibrary) Create(name string, indicatorType string, config map[string]interface{}) (Indicator, error) {
	lib.mu.RLock()
	factory, exists := lib.factories[indicatorType]
	lib.mu.RUnlock()

	if !exists {
		return nil, ErrIndicatorTypeNotFound
	}

	indicator, err := factory(config)
	if err != nil {
		return nil, err
	}

	lib.mu.Lock()
	lib.indicators[name] = indicator
	lib.mu.Unlock()

	return indicator, nil
}

// Get retrieves an indicator by name
func (lib *IndicatorLibrary) Get(name string) (Indicator, bool) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	indicator, exists := lib.indicators[name]
	return indicator, exists
}

// UpdateAll updates all indicators with new market data
func (lib *IndicatorLibrary) UpdateAll(md *mdpb.MarketDataUpdate) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	for _, indicator := range lib.indicators {
		indicator.Update(md)
	}
}

// ResetAll resets all indicators
func (lib *IndicatorLibrary) ResetAll() {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	for _, indicator := range lib.indicators {
		indicator.Reset()
	}
}

// GetAllValues returns current values of all indicators
func (lib *IndicatorLibrary) GetAllValues() map[string]float64 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	result := make(map[string]float64)
	for name, indicator := range lib.indicators {
		result[name] = indicator.GetValue()
	}
	return result
}

// MarketDataSnapshot represents a point-in-time snapshot of market data
type MarketDataSnapshot struct {
	Symbol    string
	Exchange  string
	Timestamp time.Time
	MidPrice  float64
	BidPrice  float64
	AskPrice  float64
	BidVolume uint32
	AskVolume uint32
	LastPrice float64
	Volume    uint64
	Turnover  float64
}

// FromMarketDataUpdate creates a snapshot from market data update
func FromMarketDataUpdate(md *mdpb.MarketDataUpdate) *MarketDataSnapshot {
	snapshot := &MarketDataSnapshot{
		Symbol:    md.Symbol,
		Exchange:  md.Exchange,
		Timestamp: time.Unix(0, int64(md.Timestamp)),
		Volume:    md.TotalVolume,
		Turnover:  md.Turnover,
	}

	if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
		snapshot.BidPrice = md.BidPrice[0]
		snapshot.AskPrice = md.AskPrice[0]
		snapshot.MidPrice = (md.BidPrice[0] + md.AskPrice[0]) / 2.0
	}

	if len(md.BidQty) > 0 && len(md.AskQty) > 0 {
		snapshot.BidVolume = md.BidQty[0]
		snapshot.AskVolume = md.AskQty[0]
	}

	snapshot.LastPrice = md.LastPrice

	return snapshot
}
