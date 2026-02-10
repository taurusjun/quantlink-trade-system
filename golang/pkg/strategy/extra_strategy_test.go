package strategy

import (
	"math"
	"testing"
)

// floatEquals compares two floating-point numbers with tolerance
func floatEquals(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestExtraStrategy_Creation(t *testing.T) {
	instru := &Instrument{
		Symbol:   "TEST",
		TickSize: 1.0,
		LotSize:  10,
	}
	es := NewExtraStrategy(1, instru)

	if es == nil {
		t.Fatal("ExtraStrategy should not be nil")
	}

	if es.StrategyID != 1 {
		t.Errorf("Expected StrategyID 1, got %d", es.StrategyID)
	}

	if es.Instru.Symbol != "TEST" {
		t.Errorf("Expected Symbol TEST, got %s", es.Instru.Symbol)
	}

	if !es.Active {
		t.Error("ExtraStrategy should be active by default")
	}

	if es.OrdMap == nil {
		t.Error("OrdMap should be initialized")
	}
}

func TestExtraStrategy_ProcessTrade_Buy(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order to order map
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.OrdType = OrderHitTypeStandard
	es.AddToOrderMap(orderStats)

	// Process trade
	es.ProcessTrade(100, 5, 1000.0, TransactionTypeBuy)

	// Check position update
	if es.NetPos != 5 {
		t.Errorf("Expected NetPos 5, got %d", es.NetPos)
	}

	if es.NetPosPass != 5 {
		t.Errorf("Expected NetPosPass 5 (passive order), got %d", es.NetPosPass)
	}

	if es.NetPosAgg != 0 {
		t.Errorf("Expected NetPosAgg 0, got %d", es.NetPosAgg)
	}

	if es.BuyTotalQty != 5 {
		t.Errorf("Expected BuyTotalQty 5, got %.0f", es.BuyTotalQty)
	}
}

func TestExtraStrategy_ProcessTrade_Aggressive(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add aggressive (CROSS) order to order map
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeSell
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.OrdType = OrderHitTypeCross // Aggressive order
	es.AddToOrderMap(orderStats)

	// Process trade
	es.ProcessTrade(100, 5, 1000.0, TransactionTypeSell)

	// Check position update
	if es.NetPos != -5 {
		t.Errorf("Expected NetPos -5, got %d", es.NetPos)
	}

	if es.NetPosPass != 0 {
		t.Errorf("Expected NetPosPass 0, got %d", es.NetPosPass)
	}

	if es.NetPosAgg != -5 {
		t.Errorf("Expected NetPosAgg -5 (aggressive order), got %d", es.NetPosAgg)
	}
}

func TestExtraStrategy_SetQuantAhead_Trade(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order to order map
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.QuantAhead = 50.0
	orderStats.QuantBehind = 30.0
	es.AddToOrderMap(orderStats)

	// Simulate trade at same price level (reduces quantAhead)
	es.SetQuantAhead(MarketUpdateTypeTrade, 1000.0, 10, 0, TransactionTypeBuy)

	// Check quantAhead was reduced
	if orderStats.QuantAhead != 40.0 {
		t.Errorf("Expected QuantAhead 40.0 after trade, got %.1f", orderStats.QuantAhead)
	}

	// QuantBehind should remain unchanged
	if orderStats.QuantBehind != 30.0 {
		t.Errorf("Expected QuantBehind 30.0, got %.1f", orderStats.QuantBehind)
	}
}

func TestExtraStrategy_SetQuantAhead_Add(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order to order map
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeSell
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.QuantAhead = 20.0
	orderStats.QuantBehind = 10.0
	es.AddToOrderMap(orderStats)

	// Simulate new order added (increases quantBehind)
	es.SetQuantAhead(MarketUpdateTypeAdd, 1000.0, 15, 0, TransactionTypeSell)

	// Check quantBehind was increased
	if orderStats.QuantBehind != 25.0 {
		t.Errorf("Expected QuantBehind 25.0 after add, got %.1f", orderStats.QuantBehind)
	}

	// QuantAhead should remain unchanged
	if orderStats.QuantAhead != 20.0 {
		t.Errorf("Expected QuantAhead 20.0, got %.1f", orderStats.QuantAhead)
	}
}

func TestExtraStrategy_SetQuantAhead_Delete(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order to order map
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.QuantAhead = 50.0
	orderStats.QuantBehind = 50.0
	es.AddToOrderMap(orderStats)

	// Simulate delete - proportionally reduces ahead and behind
	es.SetQuantAhead(MarketUpdateTypeDelete, 1000.0, 20, 0, TransactionTypeBuy)

	// Should be proportionally distributed (50/50 ratio)
	// diffQty=20, total=100, ahead=50%, behind=50%
	// behindQty = (50/100) * 20 = 10
	// aheadQty = 20 - 10 = 10
	if orderStats.QuantAhead != 40.0 {
		t.Errorf("Expected QuantAhead 40.0 after delete, got %.1f", orderStats.QuantAhead)
	}
	if orderStats.QuantBehind != 40.0 {
		t.Errorf("Expected QuantBehind 40.0 after delete, got %.1f", orderStats.QuantBehind)
	}
}

func TestExtraStrategy_InitQuantAhead(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	es.OrdMap[100] = orderStats

	// Initialize with 100 total qty at price, position 0.3 (30% ahead)
	es.InitQuantAhead(100, 100, 0.3)

	// totalQty = 100 - 10 (our order) = 90
	// QuantAhead = 90 * 0.3 = 27
	// QuantBehind = 90 * 0.7 = 63
	// Note: Use tolerance for floating-point comparison due to precision issues
	if !floatEquals(orderStats.QuantAhead, 27.0, 0.01) {
		t.Errorf("Expected QuantAhead ~27.0, got %.2f", orderStats.QuantAhead)
	}
	if !floatEquals(orderStats.QuantBehind, 63.0, 0.01) {
		t.Errorf("Expected QuantBehind ~63.0, got %.2f", orderStats.QuantBehind)
	}
}

func TestExtraStrategy_HandleSquareoff(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add some orders
	order1 := NewOrderStats()
	order1.OrderID = 100
	order1.Side = TransactionTypeBuy
	order1.Price = 1000.0
	order1.Active = true
	es.AddToOrderMap(order1)

	order2 := NewOrderStats()
	order2.OrderID = 101
	order2.Side = TransactionTypeSell
	order2.Price = 1001.0
	order2.Active = true
	es.AddToOrderMap(order2)

	// Trigger squareoff
	es.HandleSquareoff()

	// Check OnFlat is set
	if !es.OnFlat {
		t.Error("Expected OnFlat to be true after HandleSquareoff")
	}

	// Check all orders are marked for cancel
	if !order1.Cancel {
		t.Error("Expected order1 to be marked for cancel")
	}
	if !order2.Cancel {
		t.Error("Expected order2 to be marked for cancel")
	}
}

func TestExtraStrategy_CalcPendingNetposAgg(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add passive order (should not count)
	passiveOrder := NewOrderStats()
	passiveOrder.OrderID = 100
	passiveOrder.Side = TransactionTypeBuy
	passiveOrder.Price = 1000.0
	passiveOrder.OpenQty = 10
	passiveOrder.OrdType = OrderHitTypeStandard
	es.AddToOrderMap(passiveOrder)

	// Add aggressive buy order (should count positive)
	aggBuyOrder := NewOrderStats()
	aggBuyOrder.OrderID = 101
	aggBuyOrder.Side = TransactionTypeBuy
	aggBuyOrder.Price = 1001.0
	aggBuyOrder.OpenQty = 5
	aggBuyOrder.OrdType = OrderHitTypeCross
	es.AddToOrderMap(aggBuyOrder)

	// Add aggressive sell order (should count negative)
	aggSellOrder := NewOrderStats()
	aggSellOrder.OrderID = 102
	aggSellOrder.Side = TransactionTypeSell
	aggSellOrder.Price = 999.0
	aggSellOrder.OpenQty = 3
	aggSellOrder.OrdType = OrderHitTypeCross
	es.AddToOrderMap(aggSellOrder)

	// Calculate pending aggressive net position
	pending := es.CalcPendingNetposAgg()

	// Expected: +5 (buy) - 3 (sell) = 2
	if pending != 2 {
		t.Errorf("Expected CalcPendingNetposAgg 2, got %d", pending)
	}
}

func TestExtraStrategy_ProcessModifyConfirm(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order at original price
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Qty = 10
	orderStats.OpenQty = 10
	orderStats.Active = true
	es.AddToOrderMap(orderStats)

	// Process modify confirmation with new price
	es.ProcessModifyConfirm(100, 1001.0, 15)

	// Check order was updated
	if orderStats.Price != 1001.0 {
		t.Errorf("Expected Price 1001.0, got %.1f", orderStats.Price)
	}
	if orderStats.Qty != 15 {
		t.Errorf("Expected Qty 15, got %d", orderStats.Qty)
	}

	// Check price maps were updated
	if _, exists := es.BidMap[1000.0]; exists {
		t.Error("Old price should be removed from BidMap")
	}
	if _, exists := es.BidMap[1001.0]; !exists {
		t.Error("New price should be added to BidMap")
	}
}

func TestExtraStrategy_ProcessCancelReject(t *testing.T) {
	instru := &Instrument{Symbol: "TEST", TickSize: 1.0, LotSize: 1}
	es := NewExtraStrategy(1, instru)

	// Add order
	orderStats := NewOrderStats()
	orderStats.OrderID = 100
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = 1000.0
	orderStats.Active = true
	orderStats.Cancel = true // Cancel was requested
	es.AddToOrderMap(orderStats)

	// Process cancel rejection
	es.ProcessCancelReject(100)

	// Check cancel flag was reset
	if orderStats.Cancel {
		t.Error("Cancel flag should be reset after reject")
	}
	if orderStats.Status != OrderStatusCancelReject {
		t.Errorf("Expected status CancelReject, got %v", orderStats.Status)
	}
	if es.RejectCount != 1 {
		t.Errorf("Expected RejectCount 1, got %d", es.RejectCount)
	}
	if es.LastCancelRejectOrderID != 100 {
		t.Errorf("Expected LastCancelRejectOrderID 100, got %d", es.LastCancelRejectOrderID)
	}
}
