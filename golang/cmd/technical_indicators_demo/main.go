package main

import (
	"fmt"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func main() {
	fmt.Println("==========================================")
	fmt.Println("技术指标演示程序")
	fmt.Println("Technical Indicators Demo")
	fmt.Println("==========================================")
	fmt.Println()

	// Create indicator library
	lib := indicators.NewIndicatorLibrary()

	// Create all 10 technical indicators
	configs := map[string]map[string]interface{}{
		"sma_20": {
			"period":      float64(20),
			"max_history": float64(100),
		},
		"ema_20": {
			"period":      float64(20),
			"max_history": float64(100),
		},
		"wma_20": {
			"period":      float64(20),
			"max_history": float64(100),
		},
		"rsi_14": {
			"period":      float64(14),
			"max_history": float64(100),
		},
		"macd": {
			"fast_period": float64(12),
			"slow_period": float64(26),
			"signal_period": float64(9),
			"max_history": float64(100),
		},
		"bollinger_20": {
			"period":      float64(20),
			"num_std_dev": float64(2),
			"max_history": float64(100),
		},
		"atr_14": {
			"period":      float64(14),
			"max_history": float64(100),
		},
		"momentum_10": {
			"period":      float64(10),
			"max_history": float64(100),
		},
		"roc_10": {
			"period":      float64(10),
			"max_history": float64(100),
		},
		"stddev_20": {
			"period":      float64(20),
			"max_history": float64(100),
		},
	}

	// Create indicators
	for name, config := range configs {
		var indicatorType string
		switch {
		case name == "sma_20":
			indicatorType = "sma"
		case name == "ema_20":
			indicatorType = "ewma"
		case name == "wma_20":
			indicatorType = "wma"
		case name == "rsi_14":
			indicatorType = "rsi"
		case name == "macd":
			indicatorType = "macd"
		case name == "bollinger_20":
			indicatorType = "bollinger_bands"
		case name == "atr_14":
			indicatorType = "atr"
		case name == "momentum_10":
			indicatorType = "momentum"
		case name == "roc_10":
			indicatorType = "roc"
		case name == "stddev_20":
			indicatorType = "stddev"
		}

		_, err := lib.Create(name, indicatorType, config)
		if err != nil {
			fmt.Printf("创建指标失败 %s: %v\n", name, err)
			return
		}
	}

	fmt.Println("已创建 10 个技术指标:")
	fmt.Println("1. SMA(20) - 简单移动平均")
	fmt.Println("2. EMA(20) - 指数移动平均")
	fmt.Println("3. WMA(20) - 加权移动平均")
	fmt.Println("4. RSI(14) - 相对强弱指标")
	fmt.Println("5. MACD(12,26,9) - 移动平均收敛发散")
	fmt.Println("6. BollingerBands(20,2) - 布林带")
	fmt.Println("7. ATR(14) - 平均真实范围")
	fmt.Println("8. Momentum(10) - 动量指标")
	fmt.Println("9. ROC(10) - 变动率")
	fmt.Println("10. StdDev(20) - 标准差")
	fmt.Println()

	// Simulate market data updates with different trend scenarios
	fmt.Println("模拟市场数据更新...")
	fmt.Println()

	scenarios := []struct {
		name   string
		prices []float64
	}{
		{
			name: "场景 1: 强劲上升趋势",
			prices: []float64{
				100, 102, 104, 106, 108, 110, 112, 114, 116, 118,
				120, 122, 124, 126, 128, 130, 132, 134, 136, 138,
				140, 142, 144, 146, 148, 150,
			},
		},
		{
			name: "场景 2: 下降趋势",
			prices: []float64{
				150, 148, 146, 144, 142, 140, 138, 136, 134, 132,
				130, 128, 126, 124, 122, 120, 118, 116, 114, 112,
				110, 108, 106, 104, 102, 100,
			},
		},
		{
			name: "场景 3: 高波动震荡",
			prices: []float64{
				100, 110, 95, 115, 90, 120, 85, 125, 95, 115,
				90, 120, 95, 115, 100, 110, 95, 115, 100, 110,
				95, 115, 100, 110, 100, 105,
			},
		},
		{
			name: "场景 4: 低波动盘整",
			prices: []float64{
				100, 101, 100, 99, 100, 101, 100, 99, 100, 101,
				100, 99, 100, 101, 100, 99, 100, 101, 100, 99,
				100, 101, 100, 99, 100, 100,
			},
		},
	}

	for scenarioIdx, scenario := range scenarios {
		fmt.Printf("========== %s ==========\n", scenario.name)

		// Feed all prices to indicators
		for _, price := range scenario.prices {
			md := &mdpb.MarketDataUpdate{
				Symbol:      "AG2502",
				Exchange:    "SHFE",
				Timestamp:   uint64(time.Now().UnixNano()),
				BidPrice:    []float64{price - 0.5},
				AskPrice:    []float64{price + 0.5},
				LastPrice:   price,
				TotalVolume: 1000,
			}
			lib.UpdateAll(md)
		}

		// Display indicator values
		values := lib.GetAllValues()

		fmt.Printf("最新价格: %.2f\n", scenario.prices[len(scenario.prices)-1])
		fmt.Println()
		fmt.Println("技术指标值:")
		fmt.Printf("  1. SMA(20):       %.2f\n", values["sma_20"])
		fmt.Printf("  2. EMA(20):       %.2f\n", values["ema_20"])
		fmt.Printf("  3. WMA(20):       %.2f\n", values["wma_20"])
		fmt.Printf("  4. RSI(14):       %.2f\n", values["rsi_14"])
		fmt.Printf("  5. MACD:          %.4f\n", values["macd"])
		fmt.Printf("  6. Bollinger:     %.2f\n", values["bollinger_20"])
		fmt.Printf("  7. ATR(14):       %.4f\n", values["atr_14"])
		fmt.Printf("  8. Momentum(10):  %.2f\n", values["momentum_10"])
		fmt.Printf("  9. ROC(10):       %.2f%%\n", values["roc_10"])
		fmt.Printf(" 10. StdDev(20):    %.4f\n", values["stddev_20"])
		fmt.Println()

		// Trend analysis
		fmt.Println("趋势分析:")

		lastPrice := scenario.prices[len(scenario.prices)-1]
		sma := values["sma_20"]
		ema := values["ema_20"]
		rsi := values["rsi_14"]
		momentum := values["momentum_10"]
		roc := values["roc_10"]
		stddev := values["stddev_20"]

		// Trend direction
		if lastPrice > sma && lastPrice > ema {
			fmt.Println("  • 价格位于均线上方，上升趋势")
		} else if lastPrice < sma && lastPrice < ema {
			fmt.Println("  • 价格位于均线下方，下降趋势")
		} else {
			fmt.Println("  • 价格接近均线，趋势不明")
		}

		// RSI analysis
		if rsi > 70 {
			fmt.Printf("  • RSI = %.0f，超买区域\n", rsi)
		} else if rsi < 30 {
			fmt.Printf("  • RSI = %.0f，超卖区域\n", rsi)
		} else {
			fmt.Printf("  • RSI = %.0f，中性区域\n", rsi)
		}

		// Momentum analysis
		if momentum > 10 {
			fmt.Printf("  • 动量 = %.0f，强劲上涨动能\n", momentum)
		} else if momentum < -10 {
			fmt.Printf("  • 动量 = %.0f，强劲下跌动能\n", momentum)
		} else {
			fmt.Printf("  • 动量 = %.0f，动能较弱\n", momentum)
		}

		// ROC analysis
		if roc > 5 {
			fmt.Printf("  • ROC = %.1f%%，快速上涨\n", roc)
		} else if roc < -5 {
			fmt.Printf("  • ROC = %.1f%%，快速下跌\n", roc)
		} else {
			fmt.Printf("  • ROC = %.1f%%，变化平缓\n", roc)
		}

		// Volatility analysis
		if stddev > 10 {
			fmt.Printf("  • 标准差 = %.2f，高波动\n", stddev)
		} else if stddev < 3 {
			fmt.Printf("  • 标准差 = %.2f，低波动\n", stddev)
		} else {
			fmt.Printf("  • 标准差 = %.2f，正常波动\n", stddev)
		}

		fmt.Println()

		if scenarioIdx < len(scenarios)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("==========================================")
	fmt.Println("演示完成")
	fmt.Println("==========================================")
}
