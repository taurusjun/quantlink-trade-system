package strategy

import (
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// Strategy is the interface that all trading strategies must implement
type Strategy interface {
	// GetID returns the unique strategy ID
	GetID() string

	// GetType returns the strategy type name
	GetType() string

	// Initialize initializes the strategy with configuration
	Initialize(config *StrategyConfig) error

	// Start starts the strategy
	Start() error

	// Stop stops the strategy
	Stop() error

	// IsRunning returns true if strategy is running
	IsRunning() bool

	// OnMarketData is called when new market data arrives
	OnMarketData(md *mdpb.MarketDataUpdate)

	// OnOrderUpdate is called when order status changes
	OnOrderUpdate(update *orspb.OrderUpdate)

	// OnTimer is called periodically
	OnTimer(now time.Time)

	// GetSignals returns pending trading signals
	GetSignals() []*TradingSignal

	// GetPosition returns current position
	GetPosition() *Position

	// GetPNL returns current P&L
	GetPNL() *PNL

	// GetRiskMetrics returns current risk metrics
	GetRiskMetrics() *RiskMetrics

	// GetStatus returns strategy status
	GetStatus() *StrategyStatus

	// Reset resets the strategy to initial state
	Reset()
}

// BaseStrategy provides common functionality for all strategies
type BaseStrategy struct {
	ID              string
	Type            string
	Config          *StrategyConfig
	Indicators      *indicators.IndicatorLibrary
	Position        *Position
	PNL             *PNL
	RiskMetrics     *RiskMetrics
	Status          *StrategyStatus
	IsRunningFlag   bool
	PendingSignals  []*TradingSignal
	Orders          map[string]*orspb.OrderUpdate // order_id -> OrderUpdate
}

// NewBaseStrategy creates a new base strategy
func NewBaseStrategy(id string, strategyType string) *BaseStrategy {
	return &BaseStrategy{
		ID:             id,
		Type:           strategyType,
		Indicators:     indicators.NewIndicatorLibrary(),
		Position:       &Position{},
		PNL:            &PNL{},
		RiskMetrics:    &RiskMetrics{},
		Status:         &StrategyStatus{StrategyID: id},
		PendingSignals: make([]*TradingSignal, 0),
		Orders:         make(map[string]*orspb.OrderUpdate),
	}
}

// GetID returns strategy ID
func (bs *BaseStrategy) GetID() string {
	return bs.ID
}

// GetType returns strategy type
func (bs *BaseStrategy) GetType() string {
	return bs.Type
}

// IsRunning returns true if strategy is running
func (bs *BaseStrategy) IsRunning() bool {
	return bs.IsRunningFlag
}

// GetPosition returns current position
func (bs *BaseStrategy) GetPosition() *Position {
	return bs.Position
}

// GetPNL returns current P&L
func (bs *BaseStrategy) GetPNL() *PNL {
	return bs.PNL
}

// GetRiskMetrics returns risk metrics
func (bs *BaseStrategy) GetRiskMetrics() *RiskMetrics {
	return bs.RiskMetrics
}

// GetStatus returns strategy status
func (bs *BaseStrategy) GetStatus() *StrategyStatus {
	bs.Status.IsRunning = bs.IsRunningFlag
	bs.Status.Position = bs.Position
	bs.Status.PNL = bs.PNL
	bs.Status.RiskMetrics = bs.RiskMetrics
	return bs.Status
}

// GetSignals returns pending signals and clears the queue
func (bs *BaseStrategy) GetSignals() []*TradingSignal {
	signals := bs.PendingSignals
	bs.PendingSignals = make([]*TradingSignal, 0)
	return signals
}

// AddSignal adds a new trading signal
func (bs *BaseStrategy) AddSignal(signal *TradingSignal) {
	bs.PendingSignals = append(bs.PendingSignals, signal)
	bs.Status.SignalCount++
	bs.Status.LastSignalTime = time.Now()
}

// UpdatePosition updates position based on order update
func (bs *BaseStrategy) UpdatePosition(update *orspb.OrderUpdate) {
	// Store order update
	bs.Orders[update.OrderId] = update

	// Update position only for filled orders
	if update.Status == orspb.OrderStatus_FILLED {
		bs.Status.FillCount++

		qty := update.FilledQty
		price := update.AvgPrice

		if update.Side == orspb.OrderSide_BUY {
			// Calculate realized PNL if closing short position
			if bs.Position.ShortQty > 0 && bs.Position.LongQty == 0 {
				// Buy is closing a short position
				closedQty := qty
				if closedQty > bs.Position.ShortQty {
					closedQty = bs.Position.ShortQty
				}
				// Short PNL: (avg_short_price - buy_price) * qty
				realizedPnL := (bs.Position.AvgShortPrice - price) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL
			}

			// Update long position (always add to long)
			totalCost := bs.Position.AvgLongPrice * float64(bs.Position.LongQty)
			totalCost += price * float64(qty)
			bs.Position.LongQty += qty
			if bs.Position.LongQty > 0 {
				bs.Position.AvgLongPrice = totalCost / float64(bs.Position.LongQty)
			}
		} else {
			// Calculate realized PNL if closing long position
			if bs.Position.LongQty > 0 && bs.Position.ShortQty == 0 {
				// Sell is closing a long position
				closedQty := qty
				if closedQty > bs.Position.LongQty {
					closedQty = bs.Position.LongQty
				}
				// Long PNL: (sell_price - avg_long_price) * qty
				realizedPnL := (price - bs.Position.AvgLongPrice) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL
			}

			// Update short position (always add to short)
			totalCost := bs.Position.AvgShortPrice * float64(bs.Position.ShortQty)
			totalCost += price * float64(qty)
			bs.Position.ShortQty += qty
			if bs.Position.ShortQty > 0 {
				bs.Position.AvgShortPrice = totalCost / float64(bs.Position.ShortQty)
			}
		}

		bs.Position.NetQty = bs.Position.LongQty - bs.Position.ShortQty
		bs.Position.LastUpdate = time.Now()
	} else if update.Status == orspb.OrderStatus_REJECTED {
		bs.Status.RejectCount++
	}
}

// UpdatePNL updates P&L based on current market price
func (bs *BaseStrategy) UpdatePNL(currentPrice float64) {
	if bs.Position.IsFlat() {
		bs.PNL.UnrealizedPnL = 0
	} else if bs.Position.IsLong() {
		// Long position: (current - avg_buy) * qty
		bs.PNL.UnrealizedPnL = (currentPrice - bs.Position.AvgLongPrice) * float64(bs.Position.LongQty)
	} else {
		// Short position: (avg_sell - current) * qty
		bs.PNL.UnrealizedPnL = (bs.Position.AvgShortPrice - currentPrice) * float64(bs.Position.ShortQty)
	}

	bs.PNL.TotalPnL = bs.PNL.RealizedPnL + bs.PNL.UnrealizedPnL
	bs.PNL.NetPnL = bs.PNL.TotalPnL - bs.PNL.TradingFees
	bs.PNL.Timestamp = time.Now()
}

// UpdateRiskMetrics updates risk metrics
func (bs *BaseStrategy) UpdateRiskMetrics(currentPrice float64) {
	bs.RiskMetrics.PositionSize = abs(bs.Position.NetQty)
	bs.RiskMetrics.ExposureValue = float64(bs.RiskMetrics.PositionSize) * currentPrice
	bs.RiskMetrics.Timestamp = time.Now()

	// Update max drawdown
	if bs.PNL.TotalPnL < 0 && absFloat(bs.PNL.TotalPnL) > bs.RiskMetrics.MaxDrawdown {
		bs.RiskMetrics.MaxDrawdown = absFloat(bs.PNL.TotalPnL)
	}
}

// CheckRiskLimits checks if risk limits are exceeded
func (bs *BaseStrategy) CheckRiskLimits() bool {
	if bs.Config == nil {
		return true
	}

	// Check position size limit
	if bs.Config.MaxPositionSize > 0 {
		if absInt64(bs.Position.NetQty) > bs.Config.MaxPositionSize {
			return false
		}
	}

	// Check exposure limit
	if bs.Config.MaxExposure > 0 {
		if bs.RiskMetrics.ExposureValue > bs.Config.MaxExposure {
			return false
		}
	}

	// Check drawdown limit
	if maxDrawdown, ok := bs.Config.RiskLimits["max_drawdown"]; ok {
		if bs.RiskMetrics.MaxDrawdown > maxDrawdown {
			return false
		}
	}

	return true
}

// Reset resets the strategy
func (bs *BaseStrategy) Reset() {
	bs.Position = &Position{}
	bs.PNL = &PNL{}
	bs.RiskMetrics = &RiskMetrics{}
	bs.PendingSignals = make([]*TradingSignal, 0)
	bs.Orders = make(map[string]*orspb.OrderUpdate)
	bs.Indicators.ResetAll()
}

// Helper functions
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
