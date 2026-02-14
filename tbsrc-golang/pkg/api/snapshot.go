package api

import (
	"time"

	"tbsrc-golang/pkg/execution"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/strategy"
	"tbsrc-golang/pkg/types"
)

// DashboardSnapshot 每秒推送给前端的完整快照
type DashboardSnapshot struct {
	Timestamp  string         `json:"timestamp"`
	StrategyID int32          `json:"strategy_id"`
	Active     bool           `json:"active"`
	Account    string         `json:"account"`
	Spread     SpreadSnapshot `json:"spread"`
	Leg1       LegSnapshot    `json:"leg1"`
	Leg2       LegSnapshot    `json:"leg2"`
	Exposure   int32          `json:"exposure"` // NetExposure()
}

// SpreadSnapshot 价差分析
type SpreadSnapshot struct {
	Current   float64 `json:"current"`
	AvgSpread float64 `json:"avg_spread"`
	AvgOri    float64 `json:"avg_ori"`
	TValue    float64 `json:"t_value"`
	Deviation float64 `json:"deviation"`
	IsValid   bool    `json:"is_valid"`
	Alpha     float64 `json:"alpha"`
}

// LegSnapshot 单腿完整状态
type LegSnapshot struct {
	Symbol   string `json:"symbol"`
	Exchange string `json:"exchange"`
	// 行情
	BidPx       float64 `json:"bid_px"`
	AskPx       float64 `json:"ask_px"`
	MidPx       float64 `json:"mid_px"`
	BidQty      float64 `json:"bid_qty"`
	AskQty      float64 `json:"ask_qty"`
	LastTradePx float64 `json:"last_trade_px"`
	// 持仓
	Netpos     int32 `json:"netpos"`
	NetposPass int32 `json:"netpos_pass"`
	NetposAgg  int32 `json:"netpos_agg"`
	// PNL
	RealisedPNL   float64 `json:"realised_pnl"`
	UnrealisedPNL float64 `json:"unrealised_pnl"`
	NetPNL        float64 `json:"net_pnl"`
	GrossPNL      float64 `json:"gross_pnl"`
	MaxPNL        float64 `json:"max_pnl"`
	Drawdown      float64 `json:"drawdown"`
	// 交易统计
	TradeCount   int32   `json:"trade_count"`
	OrderCount   int32   `json:"order_count"`
	RejectCount  int32   `json:"reject_count"`
	CancelCount  int32   `json:"cancel_count"`
	BuyTotalQty  float64 `json:"buy_total_qty"`
	SellTotalQty float64 `json:"sell_total_qty"`
	// 动态阈值
	TholdBidPlace  float64 `json:"thold_bid_place"`
	TholdBidRemove float64 `json:"thold_bid_remove"`
	TholdAskPlace  float64 `json:"thold_ask_place"`
	TholdAskRemove float64 `json:"thold_ask_remove"`
	TholdMaxPos    int32   `json:"thold_max_pos"`
	TholdSize      int32   `json:"thold_size"`
	// 挂单
	BuyOpenOrders  int32   `json:"buy_open_orders"`
	SellOpenOrders int32   `json:"sell_open_orders"`
	BuyOpenQty     float64 `json:"buy_open_qty"`
	SellOpenQty    float64 `json:"sell_open_qty"`
	// 状态标志
	OnExit     bool `json:"on_exit"`
	OnFlat     bool `json:"on_flat"`
	OnStopLoss bool `json:"on_stop_loss"`
	// 订单列表
	Orders []OrderSnapshot `json:"orders"`
}

// OrderSnapshot 单个挂单
type OrderSnapshot struct {
	OrderID uint32  `json:"order_id"`
	Side    string  `json:"side"`     // "BUY" / "SELL"
	Price   float64 `json:"price"`
	OpenQty int32   `json:"open_qty"`
	DoneQty int32   `json:"done_qty"`
	Status  string  `json:"status"`   // "NEW_CONFIRM", "TRADED" 等
	OrdType string  `json:"ord_type"` // "STANDARD", "CROSS" 等
	Time    string  `json:"time"`     // 时间戳（由 history tracker 填充）
}

// OrderHistoryTracker 订单历史追踪器
// OrdMap 中的订单在成交后会被移除，模拟器填单极快（~150ms），
// 但快照每秒才采集一次，大部分订单会被错过。
// 此追踪器维护一个环形缓冲区，保留最近 N 条订单（包括已完成的）。
type OrderHistoryTracker struct {
	history []OrderSnapshot // 环形缓冲区
	maxSize int
	seen    map[uint32]bool // 已经记录过的 orderID
}

// NewOrderHistoryTracker 创建订单历史追踪器
func NewOrderHistoryTracker(maxSize int) *OrderHistoryTracker {
	return &OrderHistoryTracker{
		history: make([]OrderSnapshot, 0, maxSize),
		maxSize: maxSize,
		seen:    make(map[uint32]bool),
	}
}

// Update 接收当前快照中的活跃订单，合并到历史中，返回完整列表
// 活跃订单在前（更新状态），已完成订单在后（按时间倒序）
func (t *OrderHistoryTracker) Update(liveOrders []OrderSnapshot) []OrderSnapshot {
	now := time.Now().Format("15:04:05")

	// 更新已有订单状态，添加新订单
	for _, o := range liveOrders {
		o.Time = now
		if !t.seen[o.OrderID] {
			// 新订单，加入历史
			t.seen[o.OrderID] = true
			t.history = append(t.history, o)
		} else {
			// 已有订单，更新状态
			for i := range t.history {
				if t.history[i].OrderID == o.OrderID {
					t.history[i] = o
					break
				}
			}
		}
	}

	// 标记不在 liveOrders 中的订单为 TRADED（已从 OrdMap 移除 = 已成交）
	liveSet := make(map[uint32]bool, len(liveOrders))
	for _, o := range liveOrders {
		liveSet[o.OrderID] = true
	}
	for i := range t.history {
		if !liveSet[t.history[i].OrderID] && t.history[i].Status != "TRADED" &&
			t.history[i].Status != "CANCEL_CONFIRM" && t.history[i].Status != "NEW_REJECT" {
			t.history[i].Status = "TRADED"
			t.history[i].Time = now
		}
	}

	// 裁剪：保留最近 maxSize 条
	if len(t.history) > t.maxSize {
		removed := t.history[:len(t.history)-t.maxSize]
		for _, o := range removed {
			delete(t.seen, o.OrderID)
		}
		t.history = t.history[len(t.history)-t.maxSize:]
	}

	// 返回倒序副本（最新在前）
	result := make([]OrderSnapshot, len(t.history))
	for i, o := range t.history {
		result[len(t.history)-1-i] = o
	}
	return result
}

// CollectSnapshot 从 PairwiseArbStrategy 收集快照（在策略 goroutine 中调用）
func CollectSnapshot(pas *strategy.PairwiseArbStrategy) *DashboardSnapshot {
	snap := &DashboardSnapshot{
		Timestamp:  time.Now().Format(time.RFC3339),
		StrategyID: pas.StrategyID,
		Active:     pas.Active,
		Account:    pas.Account,
		Exposure:   pas.NetExposure(),
	}

	// 价差快照
	if pas.Spread != nil {
		snap.Spread = SpreadSnapshot{
			Current:   pas.Spread.CurrSpread,
			AvgSpread: pas.Spread.AvgSpread,
			AvgOri:    pas.Spread.AvgSpreadOri,
			TValue:    pas.Spread.TValue,
			Deviation: pas.Spread.Deviation(),
			IsValid:   pas.Spread.IsValid,
			Alpha:     pas.Spread.Alpha,
		}
	}

	// Leg1 快照
	snap.Leg1 = collectLegSnapshot(pas.Inst1, pas.Leg1)

	// Leg2 快照
	snap.Leg2 = collectLegSnapshot(pas.Inst2, pas.Leg2)

	return snap
}

// collectLegSnapshot 从 LegManager 和 Instrument 收集单腿快照
func collectLegSnapshot(inst *instrument.Instrument, leg *execution.LegManager) LegSnapshot {
	s := leg.State
	ls := LegSnapshot{
		Symbol:   inst.Symbol,
		Exchange: inst.Exchange,
		// 行情
		BidPx:       inst.BidPx[0],
		AskPx:       inst.AskPx[0],
		MidPx:       inst.MidPrice(),
		BidQty:      inst.BidQty[0],
		AskQty:      inst.AskQty[0],
		LastTradePx: inst.LastTradePx,
		// 持仓
		Netpos:     s.Netpos,
		NetposPass: s.NetposPass,
		NetposAgg:  s.NetposAgg,
		// PNL
		RealisedPNL:   s.RealisedPNL,
		UnrealisedPNL: s.UnrealisedPNL,
		NetPNL:        s.NetPNL,
		GrossPNL:      s.GrossPNL,
		MaxPNL:        s.MaxPNL,
		Drawdown:      s.Drawdown,
		// 交易统计
		TradeCount:   s.TradeCount,
		OrderCount:   s.OrderCount,
		RejectCount:  s.RejectCount,
		CancelCount:  s.CancelCount,
		BuyTotalQty:  s.BuyTotalQty,
		SellTotalQty: s.SellTotalQty,
		// 动态阈值
		TholdBidPlace:  s.TholdBidPlace,
		TholdBidRemove: s.TholdBidRemove,
		TholdAskPlace:  s.TholdAskPlace,
		TholdAskRemove: s.TholdAskRemove,
		TholdMaxPos:    s.TholdMaxPos,
		TholdSize:      s.TholdSize,
		// 挂单
		BuyOpenOrders:  s.BuyOpenOrders,
		SellOpenOrders: s.SellOpenOrders,
		BuyOpenQty:     s.BuyOpenQty,
		SellOpenQty:    s.SellOpenQty,
		// 状态标志
		OnExit:     s.OnExit,
		OnFlat:     s.OnFlat,
		OnStopLoss: s.OnStopLoss,
	}

	// 复制订单 map 为 slice
	ls.Orders = collectOrderSnapshots(leg)

	return ls
}

// collectOrderSnapshots 从 OrderManager 的 OrdMap 复制订单列表
func collectOrderSnapshots(leg *execution.LegManager) []OrderSnapshot {
	if leg.Orders == nil || len(leg.Orders.OrdMap) == 0 {
		return nil
	}

	orders := make([]OrderSnapshot, 0, len(leg.Orders.OrdMap))
	for _, ord := range leg.Orders.OrdMap {
		if ord == nil {
			continue
		}
		os := OrderSnapshot{
			OrderID: ord.OrderID,
			Side:    sideString(ord.Side),
			Price:   ord.Price,
			OpenQty: ord.OpenQty,
			DoneQty: ord.DoneQty,
			Status:  statusString(ord.Status),
			OrdType: ordTypeString(ord.OrdType),
		}
		orders = append(orders, os)
	}
	return orders
}

func sideString(side types.TransactionType) string {
	switch side {
	case types.Buy:
		return "BUY"
	case types.Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

func statusString(status types.OrderStatus) string {
	switch status {
	case types.StatusNewOrder:
		return "NEW_ORDER"
	case types.StatusNewConfirm:
		return "NEW_CONFIRM"
	case types.StatusNewReject:
		return "NEW_REJECT"
	case types.StatusModifyOrder:
		return "MODIFY_ORDER"
	case types.StatusModifyConfirm:
		return "MODIFY_CONFIRM"
	case types.StatusModifyReject:
		return "MODIFY_REJECT"
	case types.StatusCancelOrder:
		return "CANCEL_ORDER"
	case types.StatusCancelConfirm:
		return "CANCEL_CONFIRM"
	case types.StatusCancelReject:
		return "CANCEL_REJECT"
	case types.StatusTraded:
		return "TRADED"
	case types.StatusInit:
		return "INIT"
	default:
		return "UNKNOWN"
	}
}

func ordTypeString(ordType types.OrderHitType) string {
	switch ordType {
	case types.HitStandard:
		return "STANDARD"
	case types.HitImprove:
		return "IMPROVE"
	case types.HitCross:
		return "CROSS"
	case types.HitDetect:
		return "DETECT"
	case types.HitMatch:
		return "MATCH"
	default:
		return "UNKNOWN"
	}
}
