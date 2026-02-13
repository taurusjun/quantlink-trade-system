package types

// OrderStats 对应 C++ struct OrderStats
// 参考: tbsrc/Strategies/include/ExecutionStrategyStructs.h:44-68
type OrderStats struct {
	Active     bool   // m_active
	New        bool   // m_new
	ModifyWait bool   // m_modifywait
	Cancel     bool   // m_cancel
	Modify     int32  // m_modify
	LastTS     uint64 // m_lastTS
	OrderID    uint32 // m_orderID
	OldQty     int32  // m_oldQty
	NewQty     int32  // m_newQty
	Qty        int32  // m_Qty
	OpenQty    int32  // m_openQty
	CxlQty    int32  // m_cxlQty
	DoneQty    int32  // m_doneQty
	QuantAhead float64 // m_quantAhead
	QuantBehind float64 // m_quantBehind
	Price      float64 // m_price
	NewPrice   float64 // m_newprice
	OldPrice   float64 // m_oldprice
	TypeOfOrder TypeOfOrder  // m_typeOfOrder
	OrdType    OrderHitType  // m_ordType
	Status     OrderStatus   // m_status
	Side       TransactionType // m_side
}

// NewOrderStats 创建初始化的 OrderStats
// C++: ExtraStrategy.cpp:238-263 中 new OrderStats() 后的初始化
func NewOrderStats(orderID uint32, side TransactionType, price float64, qty int32,
	typeOfOrder TypeOfOrder, ordType OrderHitType) *OrderStats {
	return &OrderStats{
		Active:      false,
		New:         true,
		Cancel:      false,
		ModifyWait:  false,
		Modify:      0,
		Status:      StatusNewOrder,
		Price:       price,
		Side:        side,
		OrderID:     orderID,
		Qty:         qty,
		OpenQty:     qty,
		DoneQty:     0,
		QuantBehind: 0,
		TypeOfOrder: typeOfOrder,
		OrdType:     ordType,
	}
}
