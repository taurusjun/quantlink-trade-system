package risk

import (
	"testing"
)

// TestRiskConfigLoading 测试风控配置是否正确加载
func TestRiskConfigLoading(t *testing.T) {
	// 模拟配置值
	config := &RiskManagerConfig{
		EnableGlobalLimits:    true,
		EnableStrategyLimits:  true,
		EnablePortfolioLimits: true,
		CheckIntervalMs:       100,

		// 策略级别参数
		MaxPosition:  1000,
		MaxExposure:  100000000.0,
		StopLoss:     100000000.0,
		MaxLoss:      100000000.0,
		UpnlLoss:     100000000.0,
		MaxOrders:    10000,

		// 全局级别参数
		GlobalMaxExposure:  100000000.0,
		GlobalMaxDrawdown:  100000000.0,
		GlobalMaxDailyLoss: 100000000.0,
	}

	rm := NewRiskManager(config)
	if err := rm.Initialize(); err != nil {
		t.Fatalf("Failed to initialize RiskManager: %v", err)
	}

	// 验证限制值是否正确设置
	testCases := []struct {
		name     string
		limitKey string
		expected float64
	}{
		{"StopLoss", "strategy_default_stop_loss", 100000000.0},
		{"MaxLoss", "strategy_default_max_loss", 100000000.0},
		{"MaxExposure", "strategy_default_exposure", 100000000.0},
		{"UpnlLoss", "strategy_default_upnl_loss", 100000000.0},
		{"MaxPosition", "strategy_default_position", 1000.0},
		{"MaxOrders", "strategy_default_max_orders", 10000.0},
		{"GlobalMaxExposure", "global_max_exposure", 100000000.0},
		{"GlobalMaxDrawdown", "global_max_drawdown", 100000000.0},
		{"GlobalMaxDailyLoss", "global_max_daily_loss", 100000000.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			limit, ok := rm.limits[tc.limitKey]
			if !ok {
				t.Errorf("Limit %s not found", tc.limitKey)
				return
			}
			if limit.Value != tc.expected {
				t.Errorf("Limit %s: expected %.2f, got %.2f", tc.limitKey, tc.expected, limit.Value)
			}
			t.Logf("✓ %s = %.2f (enabled=%v)", tc.limitKey, limit.Value, limit.Enabled)
		})
	}
}

// TestRiskConfigZeroValues 测试当配置值为0时的行为
func TestRiskConfigZeroValues(t *testing.T) {
	// 使用零值配置
	config := &RiskManagerConfig{
		EnableGlobalLimits:   true,
		EnableStrategyLimits: true,
	}

	rm := NewRiskManager(config)
	if err := rm.Initialize(); err != nil {
		t.Fatalf("Failed to initialize RiskManager: %v", err)
	}

	// 检查零值限制
	zeroLimits := []string{
		"strategy_default_stop_loss",
		"strategy_default_max_loss",
		"strategy_default_exposure",
		"global_max_exposure",
	}

	t.Log("当配置值为0时，以下限制将触发（这是问题所在）：")
	for _, key := range zeroLimits {
		if limit, ok := rm.limits[key]; ok {
			t.Logf("  %s = %.2f (enabled=%v) <- 会立即触发！", key, limit.Value, limit.Enabled)
		}
	}
}
