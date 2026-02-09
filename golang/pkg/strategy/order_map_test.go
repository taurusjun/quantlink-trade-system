package strategy

import (
	"testing"
)

func TestOrderPriceMap_AddAndRemove(t *testing.T) {
	m := NewOrderPriceMap()

	// Add a bid order
	order1 := &PriceOrder{
		Price:     100.0,
		OrderID:   "order1",
		Symbol:    "ag2603",
		Side:      OrderSideBuy,
		Quantity:  10,
		FilledQty: 0,
		Level:     0,
	}
	m.AddOrder(order1)

	// Verify order was added
	if !m.HasOrderAtPrice(100.0, OrderSideBuy) {
		t.Error("Expected order at price 100.0")
	}
	if m.BidCount() != 1 {
		t.Errorf("Expected 1 bid order, got %d", m.BidCount())
	}
	if m.TotalCount() != 1 {
		t.Errorf("Expected 1 total order, got %d", m.TotalCount())
	}

	// Add an ask order
	order2 := &PriceOrder{
		Price:     101.0,
		OrderID:   "order2",
		Symbol:    "ag2603",
		Side:      OrderSideSell,
		Quantity:  5,
		FilledQty: 0,
		Level:     0,
	}
	m.AddOrder(order2)

	if !m.HasOrderAtPrice(101.0, OrderSideSell) {
		t.Error("Expected order at price 101.0")
	}
	if m.AskCount() != 1 {
		t.Errorf("Expected 1 ask order, got %d", m.AskCount())
	}

	// Remove order by ID
	removed := m.RemoveOrder("order1")
	if removed == nil {
		t.Error("Expected removed order to be returned")
	}
	if removed.Price != 100.0 {
		t.Errorf("Expected removed order price 100.0, got %f", removed.Price)
	}
	if m.HasOrderAtPrice(100.0, OrderSideBuy) {
		t.Error("Order should have been removed")
	}
}

func TestOrderPriceMap_GetOrderByID(t *testing.T) {
	m := NewOrderPriceMap()

	order := &PriceOrder{
		Price:     100.0,
		OrderID:   "test-order",
		Symbol:    "ag2603",
		Side:      OrderSideBuy,
		Quantity:  10,
		FilledQty: 0,
		Level:     1,
	}
	m.AddOrder(order)

	// Get by ID
	retrieved := m.GetOrderByID("test-order")
	if retrieved == nil {
		t.Error("Expected to retrieve order")
	}
	if retrieved.Level != 1 {
		t.Errorf("Expected level 1, got %d", retrieved.Level)
	}

	// Get non-existent order
	notFound := m.GetOrderByID("non-existent")
	if notFound != nil {
		t.Error("Expected nil for non-existent order")
	}
}

func TestOrderPriceMap_GetAllPendingOrders(t *testing.T) {
	m := NewOrderPriceMap()

	// Add orders with different fill states
	m.AddOrder(&PriceOrder{
		Price:     100.0,
		OrderID:   "pending1",
		Symbol:    "ag2603",
		Side:      OrderSideBuy,
		Quantity:  10,
		FilledQty: 0,
		Level:     0,
	})
	m.AddOrder(&PriceOrder{
		Price:     101.0,
		OrderID:   "filled1",
		Symbol:    "ag2603",
		Side:      OrderSideSell,
		Quantity:  5,
		FilledQty: 5, // Fully filled
		Level:     0,
	})
	m.AddOrder(&PriceOrder{
		Price:     102.0,
		OrderID:   "partial1",
		Symbol:    "ag2603",
		Side:      OrderSideSell,
		Quantity:  10,
		FilledQty: 5, // Partially filled
		Level:     1,
	})

	pending := m.GetAllPendingOrders()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending orders, got %d", len(pending))
	}
}

func TestOrderPriceMap_UpdateFilledQty(t *testing.T) {
	m := NewOrderPriceMap()

	order := &PriceOrder{
		Price:     100.0,
		OrderID:   "test-order",
		Symbol:    "ag2603",
		Side:      OrderSideBuy,
		Quantity:  10,
		FilledQty: 0,
		Level:     0,
	}
	m.AddOrder(order)

	// Update filled qty
	ok := m.UpdateFilledQty("test-order", 5)
	if !ok {
		t.Error("Expected update to succeed")
	}

	retrieved := m.GetOrderByID("test-order")
	if retrieved.FilledQty != 5 {
		t.Errorf("Expected FilledQty 5, got %d", retrieved.FilledQty)
	}
	if retrieved.GetPendingQty() != 5 {
		t.Errorf("Expected PendingQty 5, got %d", retrieved.GetPendingQty())
	}

	// Update non-existent order
	ok = m.UpdateFilledQty("non-existent", 5)
	if ok {
		t.Error("Expected update to fail for non-existent order")
	}
}

func TestOrderPriceMap_Clear(t *testing.T) {
	m := NewOrderPriceMap()

	m.AddOrder(&PriceOrder{
		Price:   100.0,
		OrderID: "order1",
		Side:    OrderSideBuy,
	})
	m.AddOrder(&PriceOrder{
		Price:   101.0,
		OrderID: "order2",
		Side:    OrderSideSell,
	})

	if m.TotalCount() != 2 {
		t.Errorf("Expected 2 orders, got %d", m.TotalCount())
	}

	m.Clear()

	if m.TotalCount() != 0 {
		t.Errorf("Expected 0 orders after clear, got %d", m.TotalCount())
	}
	if m.BidCount() != 0 {
		t.Errorf("Expected 0 bids after clear, got %d", m.BidCount())
	}
	if m.AskCount() != 0 {
		t.Errorf("Expected 0 asks after clear, got %d", m.AskCount())
	}
}

func TestPriceOrder_IsFilled(t *testing.T) {
	tests := []struct {
		name     string
		qty      int64
		filled   int64
		expected bool
	}{
		{"not filled", 10, 0, false},
		{"partially filled", 10, 5, false},
		{"fully filled", 10, 10, true},
		{"over filled", 10, 15, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &PriceOrder{
				Quantity:  tt.qty,
				FilledQty: tt.filled,
			}
			if order.IsFilled() != tt.expected {
				t.Errorf("IsFilled() = %v, expected %v", order.IsFilled(), tt.expected)
			}
		})
	}
}

func TestOrderPriceMap_ConcurrentAccess(t *testing.T) {
	m := NewOrderPriceMap()

	// Add orders concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			order := &PriceOrder{
				Price:   float64(100 + id),
				OrderID: string(rune('a' + id)),
				Side:    OrderSideBuy,
			}
			m.AddOrder(order)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if m.TotalCount() != 10 {
		t.Errorf("Expected 10 orders, got %d", m.TotalCount())
	}
}
