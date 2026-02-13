package types

// OrderStatus 对应 C++ ExecutionStrategyStructs.h enum OrderStatus
// 参考: tbsrc/Strategies/include/ExecutionStrategyStructs.h:20
type OrderStatus int32

const (
	StatusNewOrder     OrderStatus = 0  // NEW_ORDER
	StatusNewConfirm   OrderStatus = 1  // NEW_CONFIRM
	StatusNewReject    OrderStatus = 2  // NEW_REJECT
	StatusModifyOrder  OrderStatus = 3  // MODIFY_ORDER
	StatusModifyConfirm OrderStatus = 4 // MODIFY_CONFIRM
	StatusModifyReject OrderStatus = 5  // MODIFY_REJECT
	StatusCancelOrder  OrderStatus = 6  // CANCEL_ORDER
	StatusCancelConfirm OrderStatus = 7 // CANCEL_CONFIRM
	StatusCancelReject OrderStatus = 8  // CANCEL_REJECT
	StatusTraded       OrderStatus = 9  // TRADED
	StatusInit         OrderStatus = 10 // INIT
)

// OrderHitType 对应 C++ ExecutionStrategyStructs.h enum OrderHitType
// 参考: tbsrc/Strategies/include/ExecutionStrategyStructs.h:35
type OrderHitType int32

const (
	HitStandard OrderHitType = 0 // STANDARD
	HitImprove  OrderHitType = 1 // IMPROVE
	HitCross    OrderHitType = 2 // CROSS
	HitDetect   OrderHitType = 3 // DETECT
	HitMatch    OrderHitType = 4 // MATCH
)

// TransactionType 对应 C++ ORSBase.h enum TransactionType
// 参考: tbsrc/common/include/ORSBase.h:9
type TransactionType int32

const (
	Buy  TransactionType = 1 // BUY
	Sell TransactionType = 2 // SELL
)

// TypeOfOrder 对应 C++ ORSBase.h enum TypeOfOrder
// 参考: tbsrc/common/include/ORSBase.h:15
type TypeOfOrder int32

const (
	Quote  TypeOfOrder = 0 // QUOTE
	PHedge TypeOfOrder = 1 // PHEDGE
	AHedge TypeOfOrder = 2 // AHEDGE
)
