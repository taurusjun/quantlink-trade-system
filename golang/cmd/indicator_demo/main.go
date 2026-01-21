package main

import (
	"fmt"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║         HFT Indicator Library Demo                       ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create indicator library
	lib := indicators.NewIndicatorLibrary()

	// Create EWMA indicator
	ewmaConfig := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}
	ewma, err := lib.Create("ewma_20", "ewma", ewmaConfig)
	if err != nil {
		panic(err)
	}

	// Create Order Imbalance indicator
	oiConfig := map[string]interface{}{
		"levels":        5.0,
		"volume_weight": true,
		"max_history":   100.0,
	}
	oi, err := lib.Create("order_imbalance", "order_imbalance", oiConfig)
	if err != nil {
		panic(err)
	}

	// Create VWAP indicator
	vwapConfig := map[string]interface{}{
		"reset_daily": false,
		"reset_hour":  9.0,
		"max_history": 100.0,
	}
	vwap, err := lib.Create("vwap", "vwap", vwapConfig)
	if err != nil {
		panic(err)
	}

	// Create Spread indicator
	spreadConfig := map[string]interface{}{
		"absolute":    true,
		"max_history": 100.0,
	}
	spread, err := lib.Create("spread", "spread", spreadConfig)
	if err != nil {
		panic(err)
	}

	// Create Volatility indicator
	volConfig := map[string]interface{}{
		"window":         20.0,
		"use_log_returns": true,
		"max_history":    100.0,
	}
	vol, err := lib.Create("volatility", "volatility", volConfig)
	if err != nil {
		panic(err)
	}

	fmt.Println("Created indicators:")
	fmt.Println("  - EWMA (20-period)")
	fmt.Println("  - Order Imbalance (5 levels, volume-weighted)")
	fmt.Println("  - VWAP")
	fmt.Println("  - Spread (absolute)")
	fmt.Println("  - Volatility (20-period, log returns)")
	fmt.Println()

	// Simulate market data updates
	fmt.Println("Simulating market data updates...")
	fmt.Println("────────────────────────────────────────────────────────────")

	basePrice := 7950.0
	for i := 0; i < 50; i++ {
		// Simulate price movement
		priceMove := float64(i % 10 - 5) * 2.0
		bidPrice := basePrice + priceMove - 1.0
		askPrice := basePrice + priceMove + 1.0

		// Create synthetic market data
		md := &mdpb.MarketDataUpdate{
			Symbol:      "ag2412",
			Exchange:    "SHFE",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{bidPrice, bidPrice - 2, bidPrice - 4},
			BidQty:      []uint32{100, 80, 60},
			AskPrice:    []float64{askPrice, askPrice + 2, askPrice + 4},
			AskQty:      []uint32{95, 75, 55},
			LastPrice:   bidPrice + 1.0,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    (bidPrice + 1.0) * float64(1000+i*10),
		}

		// Update all indicators
		lib.UpdateAll(md)

		// Print every 10 updates
		if i%10 == 9 {
			fmt.Printf("\nUpdate #%d (Price: %.2f, Spread: %.2f)\n", i+1, bidPrice+1.0, askPrice-bidPrice)
			fmt.Printf("  EWMA:            %.4f (ready: %v)\n", ewma.GetValue(), ewma.IsReady())
			fmt.Printf("  Order Imbalance: %.4f (ready: %v)\n", oi.GetValue(), oi.IsReady())
			fmt.Printf("  VWAP:            %.4f (ready: %v)\n", vwap.GetValue(), vwap.IsReady())
			fmt.Printf("  Spread:          %.4f (ready: %v)\n", spread.GetValue(), spread.IsReady())
			fmt.Printf("  Volatility:      %.6f (ready: %v)\n", vol.GetValue(), vol.IsReady())
		}

		// Small delay to simulate real-time updates
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("Final Indicator Values:")
	fmt.Println("════════════════════════════════════════════════════════════")

	allValues := lib.GetAllValues()
	for name, value := range allValues {
		indicator, _ := lib.Get(name)
		fmt.Printf("%-20s %.6f (ready: %v)\n", name+":", value, indicator.IsReady())
	}

	fmt.Println()
	fmt.Println("✓ Indicator library demo completed successfully!")
}
