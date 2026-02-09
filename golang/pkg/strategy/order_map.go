// Package strategy provides trading strategy interfaces and implementations
package strategy

import (
	"sync"
)

// PriceOrder represents an order at a specific price level
// C++: 对应 m_bidMap/m_askMap 中存储的订单信息
type PriceOrder struct {
	Price     float64   // 挂单价格
	OrderID   string    // 订单ID
	Symbol    string    // 合约代码
	Side      OrderSide // 买卖方向
	Quantity  int64     // 委托数量
	FilledQty int64     // 已成交数量
	Level     int       // 挂单层级 (0=一档, 1=二档, ...)
}

// GetPendingQty returns the remaining unfilled quantity
func (po *PriceOrder) GetPendingQty() int64 {
	return po.Quantity - po.FilledQty
}

// IsFilled returns true if the order is fully filled
func (po *PriceOrder) IsFilled() bool {
	return po.FilledQty >= po.Quantity
}

// OrderPriceMap manages orders indexed by price
// C++: 对应 m_bidMap (买单价格→订单) 和 m_askMap (卖单价格→订单)
// Thread-safe implementation for concurrent access
type OrderPriceMap struct {
	mu        sync.RWMutex
	bidOrders map[float64]*PriceOrder // 买单：价格 → 订单
	askOrders map[float64]*PriceOrder // 卖单：价格 → 订单
	orderByID map[string]*PriceOrder  // 订单ID → 订单 (快速查找)
}

// NewOrderPriceMap creates a new OrderPriceMap
func NewOrderPriceMap() *OrderPriceMap {
	return &OrderPriceMap{
		bidOrders: make(map[float64]*PriceOrder),
		askOrders: make(map[float64]*PriceOrder),
		orderByID: make(map[string]*PriceOrder),
	}
}

// AddOrder adds an order to the map
// C++: 在订单确认后调用，维护价格→订单映射
func (m *OrderPriceMap) AddOrder(order *PriceOrder) {
	if order == nil || order.OrderID == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to price map based on side
	if order.Side == OrderSideBuy {
		m.bidOrders[order.Price] = order
	} else {
		m.askOrders[order.Price] = order
	}

	// Add to ID map for quick lookup
	m.orderByID[order.OrderID] = order
}

// RemoveOrder removes an order from the map by ID
// C++: 在订单成交或撤单后调用
func (m *OrderPriceMap) RemoveOrder(orderID string) *PriceOrder {
	if orderID == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orderByID[orderID]
	if !exists {
		return nil
	}

	// Remove from price map
	if order.Side == OrderSideBuy {
		delete(m.bidOrders, order.Price)
	} else {
		delete(m.askOrders, order.Price)
	}

	// Remove from ID map
	delete(m.orderByID, orderID)

	return order
}

// GetOrderByID returns an order by its ID
func (m *OrderPriceMap) GetOrderByID(orderID string) *PriceOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orderByID[orderID]
}

// HasOrderAtPrice checks if there's an order at the given price for the given side
// C++: 用于避免在同一价位重复挂单
func (m *OrderPriceMap) HasOrderAtPrice(price float64, side OrderSide) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if side == OrderSideBuy {
		_, exists := m.bidOrders[price]
		return exists
	}
	_, exists := m.askOrders[price]
	return exists
}

// GetOrderAtPrice returns the order at the given price for the given side
func (m *OrderPriceMap) GetOrderAtPrice(price float64, side OrderSide) *PriceOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if side == OrderSideBuy {
		return m.bidOrders[price]
	}
	return m.askOrders[price]
}

// GetAllBidOrders returns all bid orders (copy)
func (m *OrderPriceMap) GetAllBidOrders() []*PriceOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*PriceOrder, 0, len(m.bidOrders))
	for _, order := range m.bidOrders {
		orders = append(orders, order)
	}
	return orders
}

// GetAllAskOrders returns all ask orders (copy)
func (m *OrderPriceMap) GetAllAskOrders() []*PriceOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*PriceOrder, 0, len(m.askOrders))
	for _, order := range m.askOrders {
		orders = append(orders, order)
	}
	return orders
}

// GetAllPendingOrders returns all pending (unfilled) orders
// C++: 用于批量撤单场景
func (m *OrderPriceMap) GetAllPendingOrders() []*PriceOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*PriceOrder, 0, len(m.bidOrders)+len(m.askOrders))
	for _, order := range m.bidOrders {
		if !order.IsFilled() {
			orders = append(orders, order)
		}
	}
	for _, order := range m.askOrders {
		if !order.IsFilled() {
			orders = append(orders, order)
		}
	}
	return orders
}

// BidCount returns the number of bid orders
func (m *OrderPriceMap) BidCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bidOrders)
}

// AskCount returns the number of ask orders
func (m *OrderPriceMap) AskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.askOrders)
}

// TotalCount returns the total number of orders
func (m *OrderPriceMap) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.orderByID)
}

// Clear removes all orders from the map
func (m *OrderPriceMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bidOrders = make(map[float64]*PriceOrder)
	m.askOrders = make(map[float64]*PriceOrder)
	m.orderByID = make(map[string]*PriceOrder)
}

// UpdateFilledQty updates the filled quantity for an order
func (m *OrderPriceMap) UpdateFilledQty(orderID string, filledQty int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orderByID[orderID]
	if !exists {
		return false
	}

	order.FilledQty = filledQty
	return true
}
