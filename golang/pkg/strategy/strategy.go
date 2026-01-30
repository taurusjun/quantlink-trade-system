package strategy

import (
	"fmt"
	"log"
	"sync"
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

	// OnMarketData is called when new market data arrives (continuous trading)
	OnMarketData(md *mdpb.MarketDataUpdate)

	// OnAuctionData is called when auction period market data arrives
	// This allows strategies to implement special logic for auction periods
	// (e.g., opening/closing auction, like tbsrc AuctionCallBack)
	OnAuctionData(md *mdpb.MarketDataUpdate)

	// OnOrderUpdate is called when order status changes
	OnOrderUpdate(update *orspb.OrderUpdate)

	// OnTimer is called periodically
	OnTimer(now time.Time)

	// GetSignals returns pending trading signals
	GetSignals() []*TradingSignal

	// GetEstimatedPosition returns current estimated position (NOT real CTP position!)
	GetEstimatedPosition() *EstimatedPosition

	// GetPNL returns current P&L
	GetPNL() *PNL

	// GetRiskMetrics returns current risk metrics
	GetRiskMetrics() *RiskMetrics

	// GetStatus returns strategy status
	GetStatus() *StrategyStatus

	// Reset resets the strategy to initial state
	Reset()

	// UpdateParameters updates strategy parameters (for hot reload)
	UpdateParameters(params map[string]interface{}) error

	// GetCurrentParameters returns current strategy parameters
	GetCurrentParameters() map[string]interface{}
}

// ParameterUpdatable is an interface for strategies that support hot parameter reload
type ParameterUpdatable interface {
	// ApplyParameters applies new parameters to the strategy
	// Each strategy implements this to map generic parameters to strategy-specific fields
	ApplyParameters(params map[string]interface{}) error

	// GetCurrentParameters returns current strategy parameters
	GetCurrentParameters() map[string]interface{}
}

// IndicatorAwareStrategy is an optional interface for strategies that need
// to be notified when shared indicators are updated (like tbsrc INDCallBack).
// This allows strategies to insert custom logic between indicator calculation
// and signal generation.
type IndicatorAwareStrategy interface {
	// OnIndicatorUpdate is called after shared indicators are updated for a symbol
	OnIndicatorUpdate(symbol string, indicators *indicators.IndicatorLibrary)
}

// DetailedOrderStrategy is an optional interface for strategies that need
// fine-grained order event callbacks (more granular than OnOrderUpdate).
type DetailedOrderStrategy interface {
	// OnOrderNew is called when order is confirmed by exchange
	OnOrderNew(update *orspb.OrderUpdate)

	// OnOrderFilled is called when order is filled (partially or fully)
	OnOrderFilled(update *orspb.OrderUpdate)

	// OnOrderCanceled is called when order is canceled
	OnOrderCanceled(update *orspb.OrderUpdate)

	// OnOrderRejected is called when order is rejected
	OnOrderRejected(update *orspb.OrderUpdate)
}

// BaseStrategy provides common functionality for all strategies
type BaseStrategy struct {
	ID                 string
	Type               string
	Config             *StrategyConfig
	SharedIndicators   *indicators.IndicatorLibrary // Shared indicators (read-only, updated by engine)
	PrivateIndicators  *indicators.IndicatorLibrary // Private indicators (strategy-specific)
	EstimatedPosition  *EstimatedPosition           // Estimated position (NOT real CTP position!)
	PNL                *PNL
	RiskMetrics        *RiskMetrics
	Status             *StrategyStatus
	ControlState       *StrategyControlState         // State control (aligned with tbsrc)
	PendingSignals     []*TradingSignal
	Orders             map[string]*orspb.OrderUpdate              // order_id -> OrderUpdate
	LastMarketData     map[string]*mdpb.MarketDataUpdate          // symbol -> Last market data (for WebSocket push)
	MarketDataMu       sync.RWMutex                               // Protects LastMarketData map

	// Concrete strategy instance (for parameter updates)
	// This is set by concrete strategies in their constructors
	concreteStrategy interface{}
}

// NewBaseStrategy creates a new base strategy
func NewBaseStrategy(id string, strategyType string) *BaseStrategy {
	return &BaseStrategy{
		ID:                id,
		Type:              strategyType,
		PrivateIndicators: indicators.NewIndicatorLibrary(),
		EstimatedPosition: &EstimatedPosition{}, // Estimated position (NOT real CTP!)
		PNL:               &PNL{},
		RiskMetrics:       &RiskMetrics{},
		Status:            &StrategyStatus{StrategyID: id},
		ControlState:      NewStrategyControlState(true), // Auto-activate by default
		PendingSignals:    make([]*TradingSignal, 0),
		Orders:            make(map[string]*orspb.OrderUpdate),
	}
}

// SetSharedIndicators sets the shared indicator library for this strategy
// (Called by StrategyEngine during initialization)
func (bs *BaseStrategy) SetSharedIndicators(shared *indicators.IndicatorLibrary) {
	bs.SharedIndicators = shared
}

// GetIndicator gets an indicator (tries shared first, then private)
func (bs *BaseStrategy) GetIndicator(name string) (indicators.Indicator, bool) {
	// Try shared indicators first
	if bs.SharedIndicators != nil {
		if ind, ok := bs.SharedIndicators.Get(name); ok {
			return ind, true
		}
	}

	// Try private indicators
	if bs.PrivateIndicators != nil {
		if ind, ok := bs.PrivateIndicators.Get(name); ok {
			return ind, true
		}
	}

	return nil, false
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
// 注意：running 表示"策略进程在运行"，不等于"已激活可交易"
// 对应 tbsrc：TradeBot 启动后就是 running=true，但 m_Active 可能是 false
func (bs *BaseStrategy) IsRunning() bool {
	return bs.ControlState.RunState != StrategyRunStateStopped
}

// GetEstimatedPosition returns current estimated position (NOT real CTP position!)
func (bs *BaseStrategy) GetEstimatedPosition() *EstimatedPosition {
	return bs.EstimatedPosition
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
	bs.Status.IsRunning = bs.ControlState.IsActivated() && bs.ControlState.RunState != StrategyRunStateStopped
	bs.Status.EstimatedPosition = bs.EstimatedPosition // Estimated position (NOT real CTP!)
	bs.Status.PNL = bs.PNL
	bs.Status.RiskMetrics = bs.RiskMetrics

	// Debug log
	if bs.EstimatedPosition != nil {
		log.Printf("[BaseStrategy:%s] 📊 GetStatus: EstimatedPosition Long=%d, Short=%d, Net=%d (ptr=%p)",
			bs.ID, bs.EstimatedPosition.LongQty, bs.EstimatedPosition.ShortQty,
			bs.EstimatedPosition.NetQty, bs.EstimatedPosition)
	} else {
		log.Printf("[BaseStrategy:%s] ⚠️  GetStatus: EstimatedPosition is NIL!", bs.ID)
	}

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
	log.Printf("[BaseStrategy:%s] 🔍 UpdatePosition called: OrderID=%s, Symbol=%s, Status=%v, Side=%v, FilledQty=%d",
		bs.ID, update.OrderId, update.Symbol, update.Status, update.Side, update.FilledQty)

	// Store order update
	bs.Orders[update.OrderId] = update

	// Update position only for filled orders
	if update.Status == orspb.OrderStatus_FILLED {
		bs.Status.FillCount++

		qty := update.FilledQty
		price := update.AvgPrice

		if update.Side == orspb.OrderSide_BUY {
			// Calculate realized PNL if closing short position
			if bs.EstimatedPosition.ShortQty > 0 && bs.EstimatedPosition.LongQty == 0 {
				// Buy is closing a short position
				closedQty := qty
				if closedQty > bs.EstimatedPosition.ShortQty {
					closedQty = bs.EstimatedPosition.ShortQty
				}
				// Short PNL: (avg_short_price - buy_price) * qty
				realizedPnL := (bs.EstimatedPosition.AvgShortPrice - price) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL
			}

			// Update long position (always add to long)
			totalCost := bs.EstimatedPosition.AvgLongPrice * float64(bs.EstimatedPosition.LongQty)
			totalCost += price * float64(qty)
			bs.EstimatedPosition.LongQty += qty
			if bs.EstimatedPosition.LongQty > 0 {
				bs.EstimatedPosition.AvgLongPrice = totalCost / float64(bs.EstimatedPosition.LongQty)
			}
		} else {
			// Calculate realized PNL if closing long position
			if bs.EstimatedPosition.LongQty > 0 && bs.EstimatedPosition.ShortQty == 0 {
				// Sell is closing a long position
				closedQty := qty
				if closedQty > bs.EstimatedPosition.LongQty {
					closedQty = bs.EstimatedPosition.LongQty
				}
				// Long PNL: (sell_price - avg_long_price) * qty
				realizedPnL := (price - bs.EstimatedPosition.AvgLongPrice) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL
			}

			// Update short position (always add to short)
			totalCost := bs.EstimatedPosition.AvgShortPrice * float64(bs.EstimatedPosition.ShortQty)
			totalCost += price * float64(qty)
			bs.EstimatedPosition.ShortQty += qty
			if bs.EstimatedPosition.ShortQty > 0 {
				bs.EstimatedPosition.AvgShortPrice = totalCost / float64(bs.EstimatedPosition.ShortQty)
			}
		}

		bs.EstimatedPosition.NetQty = bs.EstimatedPosition.LongQty - bs.EstimatedPosition.ShortQty
		bs.EstimatedPosition.LastUpdate = time.Now()

		log.Printf("[BaseStrategy:%s] ✅ EstimatedPosition UPDATED: Long=%d, Short=%d, Net=%d, AvgLong=%.2f, AvgShort=%.2f",
			bs.ID, bs.EstimatedPosition.LongQty, bs.EstimatedPosition.ShortQty, bs.EstimatedPosition.NetQty,
			bs.EstimatedPosition.AvgLongPrice, bs.EstimatedPosition.AvgShortPrice)
	} else if update.Status == orspb.OrderStatus_REJECTED {
		bs.Status.RejectCount++
	}
}

// UpdatePNL updates P&L based on current market price
func (bs *BaseStrategy) UpdatePNL(currentPrice float64) {
	if bs.EstimatedPosition.IsFlat() {
		bs.PNL.UnrealizedPnL = 0
	} else if bs.EstimatedPosition.IsLong() {
		// Long position: (current - avg_buy) * qty
		bs.PNL.UnrealizedPnL = (currentPrice - bs.EstimatedPosition.AvgLongPrice) * float64(bs.EstimatedPosition.LongQty)
	} else {
		// Short position: (avg_sell - current) * qty
		bs.PNL.UnrealizedPnL = (bs.EstimatedPosition.AvgShortPrice - currentPrice) * float64(bs.EstimatedPosition.ShortQty)
	}

	bs.PNL.TotalPnL = bs.PNL.RealizedPnL + bs.PNL.UnrealizedPnL
	bs.PNL.NetPnL = bs.PNL.TotalPnL - bs.PNL.TradingFees
	bs.PNL.Timestamp = time.Now()
}

// UpdateRiskMetrics updates risk metrics
func (bs *BaseStrategy) UpdateRiskMetrics(currentPrice float64) {
	bs.RiskMetrics.PositionSize = abs(bs.EstimatedPosition.NetQty)
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
		if absInt64(bs.EstimatedPosition.NetQty) > bs.Config.MaxPositionSize {
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
	bs.EstimatedPosition = &EstimatedPosition{}
	bs.PNL = &PNL{}
	bs.RiskMetrics = &RiskMetrics{}
	bs.PendingSignals = make([]*TradingSignal, 0)
	bs.Orders = make(map[string]*orspb.OrderUpdate)
	bs.PrivateIndicators.ResetAll()
}

// OnAuctionData provides default implementation for auction period data
// Default behavior: Do nothing (strategies can override for auction-specific logic)
// This aligns with tbsrc AuctionCallBack concept
func (bs *BaseStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// Default: no action during auction period
	// Strategies that need auction logic should override this method
}

// UpdateParameters updates strategy parameters (hot reload support in BaseStrategy)
// This is the unified entry point for all strategies
func (bs *BaseStrategy) UpdateParameters(params map[string]interface{}) error {
	// Check if concrete strategy supports parameter updates
	if updatable, ok := bs.concreteStrategy.(ParameterUpdatable); ok {
		return updatable.ApplyParameters(params)
	}

	return fmt.Errorf("strategy %s does not implement ParameterUpdatable interface", bs.Type)
}

// GetCurrentParameters returns current strategy parameters
func (bs *BaseStrategy) GetCurrentParameters() map[string]interface{} {
	// Check if concrete strategy supports parameter queries
	if updatable, ok := bs.concreteStrategy.(ParameterUpdatable); ok {
		return updatable.GetCurrentParameters()
	}

	// Return empty map if not supported
	return make(map[string]interface{})
}

// SetConcreteStrategy sets the concrete strategy instance
// This should be called by concrete strategies in their constructors
func (bs *BaseStrategy) SetConcreteStrategy(strategy interface{}) {
	bs.concreteStrategy = strategy
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
