// Package strategy provides trading strategy implementations
package strategy

// OrderStatus represents the status of an order
// C++: ExecutionStrategyStructs.h:20-33
type OrderStatus int32

const (
	OrderStatusNewOrder      OrderStatus = iota // NEW_ORDER - 新订单已发送
	OrderStatusNewConfirm                       // NEW_CONFIRM - 新订单已确认
	OrderStatusNewReject                        // NEW_REJECT - 新订单被拒绝
	OrderStatusModifyOrder                      // MODIFY_ORDER - 改单已发送
	OrderStatusModifyConfirm                    // MODIFY_CONFIRM - 改单已确认
	OrderStatusModifyReject                     // MODIFY_REJECT - 改单被拒绝
	OrderStatusCancelOrder                      // CANCEL_ORDER - 撤单已发送
	OrderStatusCancelConfirm                    // CANCEL_CONFIRM - 撤单已确认
	OrderStatusCancelReject                     // CANCEL_REJECT - 撤单被拒绝
	OrderStatusTraded                           // TRADED - 已成交
	OrderStatusInit                             // INIT - 初始状态
)

// String returns the string representation of OrderStatus
func (s OrderStatus) String() string {
	switch s {
	case OrderStatusNewOrder:
		return "NEW_ORDER"
	case OrderStatusNewConfirm:
		return "NEW_CONFIRM"
	case OrderStatusNewReject:
		return "NEW_REJECT"
	case OrderStatusModifyOrder:
		return "MODIFY_ORDER"
	case OrderStatusModifyConfirm:
		return "MODIFY_CONFIRM"
	case OrderStatusModifyReject:
		return "MODIFY_REJECT"
	case OrderStatusCancelOrder:
		return "CANCEL_ORDER"
	case OrderStatusCancelConfirm:
		return "CANCEL_CONFIRM"
	case OrderStatusCancelReject:
		return "CANCEL_REJECT"
	case OrderStatusTraded:
		return "TRADED"
	case OrderStatusInit:
		return "INIT"
	default:
		return "UNKNOWN"
	}
}

// OrderHitType represents the type of order execution
// C++: ExecutionStrategyStructs.h:35-42
type OrderHitType int32

const (
	OrderHitTypeStandard OrderHitType = iota // STANDARD - 被动挂单
	OrderHitTypeImprove                       // IMPROVE - 改价
	OrderHitTypeCross                         // CROSS - 主动吃单
	OrderHitTypeDetect                        // DETECT - 检测单
	OrderHitTypeMatch                         // MATCH - 配对单
)

// String returns the string representation of OrderHitType
func (t OrderHitType) String() string {
	switch t {
	case OrderHitTypeStandard:
		return "STANDARD"
	case OrderHitTypeImprove:
		return "IMPROVE"
	case OrderHitTypeCross:
		return "CROSS"
	case OrderHitTypeDetect:
		return "DETECT"
	case OrderHitTypeMatch:
		return "MATCH"
	default:
		return "UNKNOWN"
	}
}

// OrderStats represents the state and statistics of an order
// C++: ExecutionStrategyStructs.h:44-68
type OrderStats struct {
	// === 状态标志 ===
	Active     bool // m_active - 订单是否活跃
	New        bool // m_new - 是否为新订单
	ModifyWait bool // m_modifywait - 是否等待改单确认
	Cancel     bool // m_cancel - 是否已撤单

	// === 计数器 ===
	Modify int32 // m_modify - 改单次数

	// === 时间戳 ===
	LastTS uint64 // m_lastTS - 最后更新时间戳

	// === 订单标识 ===
	OrderID uint32 // m_orderID - 订单ID

	// === 数量字段 ===
	OldQty  int32 // m_oldQty - 旧数量（改单前）
	NewQty  int32 // m_newQty - 新数量（改单后）
	Qty     int32 // m_Qty - 当前委托数量
	OpenQty int32 // m_openQty - 待成交数量
	CxlQty  int32 // m_cxlQty - 撤单数量
	DoneQty int32 // m_doneQty - 已成交数量

	// === 队列位置估计 ===
	QuantAhead  float64 // m_quantAhead - 前方排队量
	QuantBehind float64 // m_quantBehind - 后方排队量

	// === 价格字段 ===
	Price    float64 // m_price - 当前价格
	NewPrice float64 // m_newprice - 新价格（改单后）
	OldPrice float64 // m_oldprice - 旧价格（改单前）

	// === 订单类型 ===
	TypeOfOrder TypeOfOrder  // m_typeOfOrder - 订单开平类型
	OrdType     OrderHitType // m_ordType - 订单执行类型
	Status      OrderStatus  // m_status - 订单状态
	Side        TransactionType // m_side - 买卖方向
}

// NewOrderStats creates a new OrderStats with default values
func NewOrderStats() *OrderStats {
	return &OrderStats{
		Active: false,
		New:    true,
		Status: OrderStatusInit,
	}
}

// IsActive returns whether the order is active
func (os *OrderStats) IsActive() bool {
	return os.Active
}

// IsPending returns whether the order has pending quantity
func (os *OrderStats) IsPending() bool {
	return os.OpenQty > 0
}

// IsFilled returns whether the order is fully filled
func (os *OrderStats) IsFilled() bool {
	return os.DoneQty >= os.Qty && os.Qty > 0
}

// IsPartiallyFilled returns whether the order is partially filled
func (os *OrderStats) IsPartiallyFilled() bool {
	return os.DoneQty > 0 && os.DoneQty < os.Qty
}

// GetFilledRatio returns the filled ratio (0.0 - 1.0)
func (os *OrderStats) GetFilledRatio() float64 {
	if os.Qty <= 0 {
		return 0
	}
	return float64(os.DoneQty) / float64(os.Qty)
}

// UpdateOnFill updates the order statistics when a fill occurs
// C++: ExecutionStrategy::ProcessTrade()
func (os *OrderStats) UpdateOnFill(filledQty int32, price float64) {
	os.DoneQty += filledQty
	os.OpenQty -= filledQty
	if os.OpenQty < 0 {
		os.OpenQty = 0
	}
	if os.OpenQty == 0 {
		os.Active = false
		os.Status = OrderStatusTraded
	}
}

// UpdateOnCancel updates the order statistics when cancelled
// C++: ExecutionStrategy::ProcessCancelConfirm()
func (os *OrderStats) UpdateOnCancel() {
	os.CxlQty = os.OpenQty
	os.OpenQty = 0
	os.Active = false
	os.Cancel = true
	os.Status = OrderStatusCancelConfirm
}

// Clone creates a deep copy of OrderStats
func (os *OrderStats) Clone() *OrderStats {
	return &OrderStats{
		Active:      os.Active,
		New:         os.New,
		ModifyWait:  os.ModifyWait,
		Cancel:      os.Cancel,
		Modify:      os.Modify,
		LastTS:      os.LastTS,
		OrderID:     os.OrderID,
		OldQty:      os.OldQty,
		NewQty:      os.NewQty,
		Qty:         os.Qty,
		OpenQty:     os.OpenQty,
		CxlQty:      os.CxlQty,
		DoneQty:     os.DoneQty,
		QuantAhead:  os.QuantAhead,
		QuantBehind: os.QuantBehind,
		Price:       os.Price,
		NewPrice:    os.NewPrice,
		OldPrice:    os.OldPrice,
		TypeOfOrder: os.TypeOfOrder,
		OrdType:     os.OrdType,
		Status:      os.Status,
		Side:        os.Side,
	}
}
