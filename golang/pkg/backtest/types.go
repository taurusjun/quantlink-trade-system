package backtest

import (
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// ReplayMode defines the data replay mode
type ReplayMode int

const (
	// ReplayModeRealtime replays data at real-time speed (1x)
	ReplayModeRealtime ReplayMode = iota
	// ReplayModeFast replays data at accelerated speed (configurable multiplier)
	ReplayModeFast
	// ReplayModeInstant replays data instantly without delays
	ReplayModeInstant
)

// Trade represents a completed trade
type Trade struct {
	TradeID    string
	OrderID    string
	Symbol     string
	Side       orspb.OrderSide
	Price      float64
	Volume     int32
	PNL        float64
	Commission float64
	Timestamp  time.Time
}

// DailyPNL represents daily profit and loss statistics
type DailyPNL struct {
	Date       string
	PNL        float64
	Return     float64
	MaxPNL     float64
	MinPNL     float64
	TradeCount int
	Volume     int64
}

// BacktestResult contains the complete backtest results
type BacktestResult struct {
	// Basic Info
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	InitialCash float64
	FinalCash   float64

	// Trade Records
	Trades   []*Trade
	DailyPNL []*DailyPNL

	// Performance Metrics
	TotalPNL              float64
	TotalReturn           float64
	AnnualizedReturn      float64
	SharpeRatio           float64
	SortinoRatio          float64
	MaxDrawdown           float64
	MaxDrawdownDuration   time.Duration
	WinRate               float64
	ProfitFactor          float64
	CalmarRatio           float64
	AverageDailyReturn    float64
	AverageDailyVolatility float64

	// Trade Statistics
	TotalTrades     int
	WinTrades       int
	LossTrades      int
	AvgWin          float64
	AvgLoss         float64
	MaxWin          float64
	MaxLoss         float64
	AvgTradeSize    float64
	TotalCommission float64
}

// Order represents an order in backtest
type Order struct {
	OrderID   string
	Symbol    string
	Side      orspb.OrderSide
	Price     float64
	Volume    int32
	Filled    int32
	Status    orspb.OrderStatus
	Timestamp time.Time
}

// Fill represents an order fill
type Fill struct {
	OrderID   string
	Price     float64
	Volume    int32
	Timestamp time.Time
}

// MarketDataTick represents a single market data tick from CSV
type MarketDataTick struct {
	TimestampNs int64
	Symbol      string
	Exchange    string
	LastPrice   float64
	LastVolume  int32
	BidPrice1   float64
	BidVolume1  int32
	AskPrice1   float64
	AskVolume1  int32
	BidPrice2   float64
	BidVolume2  int32
	AskPrice2   float64
	AskVolume2  int32
	BidPrice3   float64
	BidVolume3  int32
	AskPrice3   float64
	AskVolume3  int32
	BidPrice4   float64
	BidVolume4  int32
	AskPrice4   float64
	AskVolume4  int32
	BidPrice5   float64
	BidVolume5  int32
	AskPrice5   float64
	AskVolume5  int32
}

// ToProtobuf converts MarketDataTick to protobuf MarketDataUpdate
func (tick *MarketDataTick) ToProtobuf() *mdpb.MarketDataUpdate {
	md := &mdpb.MarketDataUpdate{
		Symbol:    tick.Symbol,
		Exchange:  tick.Exchange,
		Timestamp: uint64(tick.TimestampNs),
		LastPrice: tick.LastPrice,
		LastQty:   uint32(tick.LastVolume),
		BidPrice:  make([]float64, 0, 5),
		BidQty:    make([]uint32, 0, 5),
		AskPrice:  make([]float64, 0, 5),
		AskQty:    make([]uint32, 0, 5),
	}

	// Add bid levels
	if tick.BidVolume1 > 0 {
		md.BidPrice = append(md.BidPrice, tick.BidPrice1)
		md.BidQty = append(md.BidQty, uint32(tick.BidVolume1))
	}
	if tick.BidVolume2 > 0 {
		md.BidPrice = append(md.BidPrice, tick.BidPrice2)
		md.BidQty = append(md.BidQty, uint32(tick.BidVolume2))
	}
	if tick.BidVolume3 > 0 {
		md.BidPrice = append(md.BidPrice, tick.BidPrice3)
		md.BidQty = append(md.BidQty, uint32(tick.BidVolume3))
	}
	if tick.BidVolume4 > 0 {
		md.BidPrice = append(md.BidPrice, tick.BidPrice4)
		md.BidQty = append(md.BidQty, uint32(tick.BidVolume4))
	}
	if tick.BidVolume5 > 0 {
		md.BidPrice = append(md.BidPrice, tick.BidPrice5)
		md.BidQty = append(md.BidQty, uint32(tick.BidVolume5))
	}

	// Add ask levels
	if tick.AskVolume1 > 0 {
		md.AskPrice = append(md.AskPrice, tick.AskPrice1)
		md.AskQty = append(md.AskQty, uint32(tick.AskVolume1))
	}
	if tick.AskVolume2 > 0 {
		md.AskPrice = append(md.AskPrice, tick.AskPrice2)
		md.AskQty = append(md.AskQty, uint32(tick.AskVolume2))
	}
	if tick.AskVolume3 > 0 {
		md.AskPrice = append(md.AskPrice, tick.AskPrice3)
		md.AskQty = append(md.AskQty, uint32(tick.AskVolume3))
	}
	if tick.AskVolume4 > 0 {
		md.AskPrice = append(md.AskPrice, tick.AskPrice4)
		md.AskQty = append(md.AskQty, uint32(tick.AskVolume4))
	}
	if tick.AskVolume5 > 0 {
		md.AskPrice = append(md.AskPrice, tick.AskPrice5)
		md.AskQty = append(md.AskQty, uint32(tick.AskVolume5))
	}

	return md
}
