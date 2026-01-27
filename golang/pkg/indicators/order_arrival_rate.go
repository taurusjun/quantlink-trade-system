package indicators

import (
	"fmt"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// OrderArrivalRate measures the rate of new orders arriving at different price levels
// 订单到达率：衡量不同价位的新增订单速率
//
// Tracks how quickly new orders appear in the orderbook
// Higher rate = more active order submission
// Can detect liquidity provision patterns
type OrderArrivalRate struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowDuration time.Duration // Time window for rate calculation
	levels         int           // Number of price levels to track

	// Previous state for detecting new orders
	prevBidPrices  []float64 // Previous bid prices
	prevBidQty     []uint32  // Previous bid quantities
	prevAskPrices  []float64 // Previous ask prices
	prevAskQty     []uint32  // Previous ask quantities

	// Arrival tracking
	bidArrivals    []int64 // Timestamps of new bid orders (nanoseconds)
	askArrivals    []int64 // Timestamps of new ask orders (nanoseconds)

	// Current state
	bidArrivalRate  float64 // Bid orders per second
	askArrivalRate  float64 // Ask orders per second
	totalArrivalRate float64 // Total orders per second
	arrivalImbalance float64 // (BidRate - AskRate) / TotalRate

	// Counters
	bidArrivals24h int // Bid arrivals in last 24 hours (for longer-term analysis)
	askArrivals24h int // Ask arrivals in last 24 hours
}

// NewOrderArrivalRate creates a new Order Arrival Rate indicator
func NewOrderArrivalRate(name string, windowDuration time.Duration, levels int, maxHistory int) *OrderArrivalRate {
	if windowDuration <= 0 {
		windowDuration = 60 * time.Second // Default: 60 seconds
	}
	if levels <= 0 {
		levels = 5 // Default: 5 levels
	}

	oar := &OrderArrivalRate{
		BaseIndicator:  NewBaseIndicator(name, maxHistory),
		windowDuration: windowDuration,
		levels:         levels,
		bidArrivals:    make([]int64, 0, 1000),
		askArrivals:    make([]int64, 0, 1000),
	}

	return oar
}

// NewOrderArrivalRateFromConfig creates an OrderArrivalRate from configuration
func NewOrderArrivalRateFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "OrderArrivalRate"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}
	windowDuration := 60 * time.Second
	if v, ok := config["window_duration_sec"]; ok {
		if fv, ok := v.(float64); ok {
			windowDuration = time.Duration(fv) * time.Second
		}
	}

	levels := 5
	if v, ok := config["levels"]; ok {
		if fv, ok := v.(float64); ok {
			levels = int(fv)
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewOrderArrivalRate(name, windowDuration, levels, maxHistory), nil
}

// Update updates the order arrival rate
func (oar *OrderArrivalRate) Update(md *mdpb.MarketDataUpdate) {
	oar.mu.Lock()
	defer oar.mu.Unlock()

	currentTime := time.Now().UnixNano()

	// Detect new orders by comparing with previous state
	if len(oar.prevBidPrices) > 0 && len(oar.prevAskPrices) > 0 {
		// Check bid side
		bidNewOrders := oar.detectNewOrders(
			md.BidPrice, md.BidQty,
			oar.prevBidPrices, oar.prevBidQty,
		)
		if bidNewOrders > 0 {
			for i := 0; i < bidNewOrders; i++ {
				oar.bidArrivals = append(oar.bidArrivals, currentTime)
			}
		}

		// Check ask side
		askNewOrders := oar.detectNewOrders(
			md.AskPrice, md.AskQty,
			oar.prevAskPrices, oar.prevAskQty,
		)
		if askNewOrders > 0 {
			for i := 0; i < askNewOrders; i++ {
				oar.askArrivals = append(oar.askArrivals, currentTime)
			}
		}
	}

	// Save current state for next update
	oar.savePreviousState(md)

	// Clean old arrivals outside window
	oar.cleanOldArrivals(currentTime)

	// Calculate arrival rates
	oar.calculateRates()

	oar.AddValue(oar.totalArrivalRate)
}

// detectNewOrders detects new orders by comparing current and previous orderbook
func (oar *OrderArrivalRate) detectNewOrders(
	currentPrices []float64, currentQty []uint32,
	prevPrices []float64, prevQty []uint32,
) int {
	newOrders := 0
	maxLevels := oar.levels
	if maxLevels > len(currentPrices) {
		maxLevels = len(currentPrices)
	}

	for i := 0; i < maxLevels; i++ {
		// New price level appeared
		if !oar.priceExists(currentPrices[i], prevPrices) {
			newOrders++
			continue
		}

		// Quantity increased at existing price level
		prevIdx := oar.findPriceIndex(currentPrices[i], prevPrices)
		if prevIdx >= 0 && prevIdx < len(prevQty) {
			if currentQty[i] > prevQty[prevIdx] {
				// Quantity increased = new order(s) arrived
				newOrders++
			}
		}
	}

	return newOrders
}

// priceExists checks if a price exists in the price list
func (oar *OrderArrivalRate) priceExists(price float64, prices []float64) bool {
	for _, p := range prices {
		if p == price {
			return true
		}
	}
	return false
}

// findPriceIndex finds the index of a price in the price list
func (oar *OrderArrivalRate) findPriceIndex(price float64, prices []float64) int {
	for i, p := range prices {
		if p == price {
			return i
		}
	}
	return -1
}

// savePreviousState saves current orderbook state for next comparison
func (oar *OrderArrivalRate) savePreviousState(md *mdpb.MarketDataUpdate) {
	maxLevels := oar.levels

	// Save bid side
	if maxLevels > len(md.BidPrice) {
		maxLevels = len(md.BidPrice)
	}
	oar.prevBidPrices = make([]float64, maxLevels)
	oar.prevBidQty = make([]uint32, maxLevels)
	for i := 0; i < maxLevels; i++ {
		oar.prevBidPrices[i] = md.BidPrice[i]
		oar.prevBidQty[i] = md.BidQty[i]
	}

	// Save ask side
	maxLevels = oar.levels
	if maxLevels > len(md.AskPrice) {
		maxLevels = len(md.AskPrice)
	}
	oar.prevAskPrices = make([]float64, maxLevels)
	oar.prevAskQty = make([]uint32, maxLevels)
	for i := 0; i < maxLevels; i++ {
		oar.prevAskPrices[i] = md.AskPrice[i]
		oar.prevAskQty[i] = md.AskQty[i]
	}
}

// cleanOldArrivals removes arrival timestamps outside the time window
func (oar *OrderArrivalRate) cleanOldArrivals(currentTime int64) {
	windowNanos := oar.windowDuration.Nanoseconds()
	cutoffTime := currentTime - windowNanos

	// Clean bid arrivals
	firstValid := 0
	for i, t := range oar.bidArrivals {
		if t >= cutoffTime {
			firstValid = i
			break
		}
	}
	if firstValid > 0 {
		oar.bidArrivals = oar.bidArrivals[firstValid:]
	}

	// Clean ask arrivals
	firstValid = 0
	for i, t := range oar.askArrivals {
		if t >= cutoffTime {
			firstValid = i
			break
		}
	}
	if firstValid > 0 {
		oar.askArrivals = oar.askArrivals[firstValid:]
	}
}

// calculateRates calculates arrival rates
func (oar *OrderArrivalRate) calculateRates() {
	if len(oar.bidArrivals) == 0 && len(oar.askArrivals) == 0 {
		oar.bidArrivalRate = 0
		oar.askArrivalRate = 0
		oar.totalArrivalRate = 0
		oar.arrivalImbalance = 0
		return
	}

	// Calculate rates (arrivals per second)
	windowSeconds := oar.windowDuration.Seconds()
	oar.bidArrivalRate = float64(len(oar.bidArrivals)) / windowSeconds
	oar.askArrivalRate = float64(len(oar.askArrivals)) / windowSeconds
	oar.totalArrivalRate = oar.bidArrivalRate + oar.askArrivalRate

	// Calculate imbalance
	if oar.totalArrivalRate > 0 {
		oar.arrivalImbalance = (oar.bidArrivalRate - oar.askArrivalRate) / oar.totalArrivalRate
	} else {
		oar.arrivalImbalance = 0
	}
}

// GetBidArrivalRate returns bid order arrival rate (orders/second)
func (oar *OrderArrivalRate) GetBidArrivalRate() float64 {
	oar.mu.RLock()
	defer oar.mu.RUnlock()
	return oar.bidArrivalRate
}

// GetAskArrivalRate returns ask order arrival rate (orders/second)
func (oar *OrderArrivalRate) GetAskArrivalRate() float64 {
	oar.mu.RLock()
	defer oar.mu.RUnlock()
	return oar.askArrivalRate
}

// GetTotalArrivalRate returns total order arrival rate (orders/second)
func (oar *OrderArrivalRate) GetTotalArrivalRate() float64 {
	oar.mu.RLock()
	defer oar.mu.RUnlock()
	return oar.totalArrivalRate
}

// GetArrivalImbalance returns arrival rate imbalance (-1 to 1)
func (oar *OrderArrivalRate) GetArrivalImbalance() float64 {
	oar.mu.RLock()
	defer oar.mu.RUnlock()
	return oar.arrivalImbalance
}

// GetActivityLevel returns order submission activity level
func (oar *OrderArrivalRate) GetActivityLevel() string {
	rate := oar.GetTotalArrivalRate()

	if rate > 5.0 {
		return "VeryHigh" // >5 new orders/sec
	} else if rate > 2.0 {
		return "High" // 2-5 orders/sec
	} else if rate > 0.5 {
		return "Medium" // 0.5-2 orders/sec
	} else if rate > 0.1 {
		return "Low" // 0.1-0.5 orders/sec
	}
	return "VeryLow" // <0.1 orders/sec
}

// GetDominantSide returns which side has more order arrivals
func (oar *OrderArrivalRate) GetDominantSide() string {
	imbalance := oar.GetArrivalImbalance()

	if imbalance > 0.3 {
		return "StrongBid" // >30% more bid orders
	} else if imbalance > 0.1 {
		return "Bid" // >10% more bid orders
	} else if imbalance < -0.3 {
		return "StrongAsk" // >30% more ask orders
	} else if imbalance < -0.1 {
		return "Ask" // >10% more ask orders
	}
	return "Balanced"
}

// GetName returns indicator name
func (oar *OrderArrivalRate) GetName() string {
	return oar.BaseIndicator.GetName()
}

// String returns a string representation
func (oar *OrderArrivalRate) String() string {
	return fmt.Sprintf("OrderArrivalRate(total=%.2f/s, bid=%.2f/s, ask=%.2f/s, imbalance=%.2f, level=%s, side=%s)",
		oar.totalArrivalRate, oar.bidArrivalRate, oar.askArrivalRate,
		oar.arrivalImbalance, oar.GetActivityLevel(), oar.GetDominantSide())
}
