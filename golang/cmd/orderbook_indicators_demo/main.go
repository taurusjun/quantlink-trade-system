package main

import (
	"fmt"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func main() {
	fmt.Println("==========================================")
	fmt.Println("订单簿指标演示程序")
	fmt.Println("Orderbook Indicators Demo")
	fmt.Println("==========================================")
	fmt.Println()

	// Create indicator library
	lib := indicators.NewIndicatorLibrary()

	// Create all 10 orderbook indicators
	configs := map[string]map[string]interface{}{
		"mid_price": {
			"max_history": float64(100),
		},
		"weighted_mid_price": {
			"levels":      float64(3),
			"max_history": float64(100),
		},
		"bid_ask_spread": {
			"absolute":    true,
			"max_history": float64(100),
		},
		"orderbook_volume_bid": {
			"levels":      float64(5),
			"side":        "bid",
			"max_history": float64(100),
		},
		"orderbook_volume_ask": {
			"levels":      float64(5),
			"side":        "ask",
			"max_history": float64(100),
		},
		"order_imbalance": {
			"levels":        float64(5),
			"volume_weight": true,
			"max_history":   float64(100),
		},
		"vwap": {
			"reset_daily": false,
			"reset_hour":  float64(9),
			"max_history": float64(100),
		},
		"price_impact_buy": {
			"volume":      float64(100),
			"side":        "buy",
			"relative":    true,
			"max_history": float64(100),
		},
		"liquidity_ratio": {
			"levels":      float64(5),
			"normalized":  true,
			"min_spread":  float64(0.0001),
			"max_history": float64(100),
		},
		"bid_ask_ratio": {
			"levels":      float64(5),
			"use_log":     false,
			"epsilon":     float64(0.01),
			"max_history": float64(100),
		},
	}

	// Create indicators
	for name, config := range configs {
		var indicatorType string
		switch name {
		case "mid_price":
			indicatorType = "mid_price"
		case "weighted_mid_price":
			indicatorType = "weighted_mid_price"
		case "bid_ask_spread":
			indicatorType = "spread"
		case "orderbook_volume_bid", "orderbook_volume_ask":
			indicatorType = "orderbook_volume"
		case "order_imbalance":
			indicatorType = "order_imbalance"
		case "vwap":
			indicatorType = "vwap"
		case "price_impact_buy":
			indicatorType = "price_impact"
		case "liquidity_ratio":
			indicatorType = "liquidity_ratio"
		case "bid_ask_ratio":
			indicatorType = "bid_ask_ratio"
		}

		_, err := lib.Create(name, indicatorType, config)
		if err != nil {
			fmt.Printf("创建指标失败 %s: %v\n", name, err)
			return
		}
	}

	fmt.Println("已创建 10 个订单簿指标:")
	fmt.Println("1. MidPrice - 中间价")
	fmt.Println("2. WeightedMidPrice - 加权中间价")
	fmt.Println("3. BidAskSpread - 买卖价差")
	fmt.Println("4. OrderBookVolume (Bid) - 买盘量")
	fmt.Println("5. OrderBookVolume (Ask) - 卖盘量")
	fmt.Println("6. OrderImbalance - 订单簿不平衡")
	fmt.Println("7. VWAP - 成交量加权平均价")
	fmt.Println("8. PriceImpact - 价格冲击")
	fmt.Println("9. LiquidityRatio - 流动性比率")
	fmt.Println("10. BidAskRatio - 买卖比率")
	fmt.Println()

	// Simulate market data updates
	fmt.Println("模拟市场数据更新...")
	fmt.Println()

	scenarios := []struct {
		name string
		md   *mdpb.MarketDataUpdate
	}{
		{
			name: "场景 1: 平衡市场",
			md: &mdpb.MarketDataUpdate{
				Symbol:      "AG2502",
				Exchange:    "SHFE",
				Timestamp:   uint64(time.Now().UnixNano()),
				BidPrice:    []float64{5000.0, 4999.0, 4998.0, 4997.0, 4996.0},
				AskPrice:    []float64{5001.0, 5002.0, 5003.0, 5004.0, 5005.0},
				BidQty:      []uint32{100, 90, 80, 70, 60},
				AskQty:      []uint32{95, 85, 75, 65, 55},
				LastPrice:   5000.5,
				TotalVolume: 1000,
			},
		},
		{
			name: "场景 2: 买盘占优",
			md: &mdpb.MarketDataUpdate{
				Symbol:      "AG2502",
				Exchange:    "SHFE",
				Timestamp:   uint64(time.Now().UnixNano()),
				BidPrice:    []float64{5001.0, 5000.0, 4999.0, 4998.0, 4997.0},
				AskPrice:    []float64{5002.0, 5003.0, 5004.0, 5005.0, 5006.0},
				BidQty:      []uint32{150, 140, 130, 120, 110},
				AskQty:      []uint32{80, 70, 60, 50, 40},
				LastPrice:   5001.5,
				TotalVolume: 1100,
			},
		},
		{
			name: "场景 3: 卖盘占优",
			md: &mdpb.MarketDataUpdate{
				Symbol:      "AG2502",
				Exchange:    "SHFE",
				Timestamp:   uint64(time.Now().UnixNano()),
				BidPrice:    []float64{4999.0, 4998.0, 4997.0, 4996.0, 4995.0},
				AskPrice:    []float64{5000.0, 5001.0, 5002.0, 5003.0, 5004.0},
				BidQty:      []uint32{70, 60, 50, 40, 30},
				AskQty:      []uint32{130, 120, 110, 100, 90},
				LastPrice:   4999.5,
				TotalVolume: 1200,
			},
		},
		{
			name: "场景 4: 高流动性",
			md: &mdpb.MarketDataUpdate{
				Symbol:      "AG2502",
				Exchange:    "SHFE",
				Timestamp:   uint64(time.Now().UnixNano()),
				BidPrice:    []float64{5000.0, 4999.5, 4999.0, 4998.5, 4998.0},
				AskPrice:    []float64{5000.5, 5001.0, 5001.5, 5002.0, 5002.5},
				BidQty:      []uint32{200, 180, 160, 140, 120},
				AskQty:      []uint32{190, 170, 150, 130, 110},
				LastPrice:   5000.2,
				TotalVolume: 1500,
			},
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("========== %s ==========\n", scenario.name)
		fmt.Printf("合约: %s, 时间戳: %d\n", scenario.md.Symbol, scenario.md.Timestamp)
		fmt.Printf("买盘: %.2f@%d, %.2f@%d, %.2f@%d\n",
			scenario.md.BidPrice[0], scenario.md.BidQty[0],
			scenario.md.BidPrice[1], scenario.md.BidQty[1],
			scenario.md.BidPrice[2], scenario.md.BidQty[2])
		fmt.Printf("卖盘: %.2f@%d, %.2f@%d, %.2f@%d\n",
			scenario.md.AskPrice[0], scenario.md.AskQty[0],
			scenario.md.AskPrice[1], scenario.md.AskQty[1],
			scenario.md.AskPrice[2], scenario.md.AskQty[2])
		fmt.Println()

		// Update all indicators
		lib.UpdateAll(scenario.md)

		// Display indicator values
		values := lib.GetAllValues()

		fmt.Println("指标值:")
		fmt.Printf("  1. 中间价:         %.4f\n", values["mid_price"])
		fmt.Printf("  2. 加权中间价:     %.4f\n", values["weighted_mid_price"])
		fmt.Printf("  3. 买卖价差:       %.4f\n", values["bid_ask_spread"])
		fmt.Printf("  4. 买盘量:         %.0f\n", values["orderbook_volume_bid"])
		fmt.Printf("  5. 卖盘量:         %.0f\n", values["orderbook_volume_ask"])
		fmt.Printf("  6. 订单不平衡:     %.4f\n", values["order_imbalance"])
		fmt.Printf("  7. VWAP:           %.4f\n", values["vwap"])
		fmt.Printf("  8. 价格冲击:       %.6f (%.4f%%)\n", values["price_impact_buy"], values["price_impact_buy"]*100)
		fmt.Printf("  9. 流动性比率:     %.2f\n", values["liquidity_ratio"])
		fmt.Printf(" 10. 买卖比率:       %.4f\n", values["bid_ask_ratio"])
		fmt.Println()

		// Analysis
		fmt.Println("市场分析:")
		imbalance := values["order_imbalance"]
		ratio := values["bid_ask_ratio"]
		liquidity := values["liquidity_ratio"]

		if imbalance > 0.1 {
			fmt.Println("  • 买盘压力较大，市场偏多")
		} else if imbalance < -0.1 {
			fmt.Println("  • 卖盘压力较大，市场偏空")
		} else {
			fmt.Println("  • 订单簿较为平衡")
		}

		if ratio > 1.2 {
			fmt.Println("  • 买盘量显著高于卖盘量")
		} else if ratio < 0.8 {
			fmt.Println("  • 卖盘量显著高于买盘量")
		}

		if liquidity > 50000 {
			fmt.Println("  • 市场流动性充足")
		} else if liquidity < 20000 {
			fmt.Println("  • 市场流动性不足，交易需谨慎")
		} else {
			fmt.Println("  • 市场流动性一般")
		}

		fmt.Println()

		if i < len(scenarios)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("==========================================")
	fmt.Println("演示完成")
	fmt.Println("==========================================")
}
