package strategy

import (
	"fmt"
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ============================================================================
// 动态阈值调整 端到端测试
// 参考: docs/cpp_reference/CODE_CONSISTENCY_CHECKLIST.md
// 参考: docs/cpp_reference/SetThresholds.cpp
// ============================================================================

// TestDynamicThreshold_E2E_FullLifecycle 完整生命周期端到端测试
// 模拟从策略启动 -> 接收行情 -> 动态调整阈值 -> 生成信号 -> 成交更新 -> 阈值再调整
func TestDynamicThreshold_E2E_FullLifecycle(t *testing.T) {
	t.Log("========================================")
	t.Log("动态阈值调整 端到端测试")
	t.Log("========================================")

	// ========================================
	// Phase 1: 策略初始化
	// ========================================
	t.Log("\n[Phase 1] 策略初始化")

	pas := NewPairwiseArbStrategy("e2e_test_001")

	// 配置与 C++ 一致: BEGIN_PLACE=2.0, LONG_PLACE=3.5, SHORT_PLACE=0.5
	// 撤单阈值: BEGIN_REMOVE=0.5, LONG_REMOVE=1.0, SHORT_REMOVE=0.2
	config := &StrategyConfig{
		StrategyID:      "e2e_test_001",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"ag2603", "ag2605"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"lookback_period":       20.0,
			"entry_zscore":          2.0,
			"exit_zscore":           0.5,
			"order_size":            10.0,
			"max_position_size":     100.0,
			// 动态阈值参数 (C++: ThresholdSet)
			"use_dynamic_threshold": true,
			"begin_zscore":          2.0, // BEGIN_PLACE
			"long_zscore":           3.5, // LONG_PLACE
			"short_zscore":          0.5, // SHORT_PLACE
			// 撤单阈值参数
			"long_exit_zscore":  1.0, // LONG_REMOVE
			"short_exit_zscore": 0.2, // SHORT_REMOVE
			// 追单参数
			"aggressive_enabled":     true,
			"aggressive_interval_ms": 100.0,
			"aggressive_max_retry":   4.0,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("策略初始化失败: %v", err)
	}

	// 验证配置加载
	t.Logf("  配置加载完成:")
	t.Logf("    beginZScore = %.2f (C++: BEGIN_PLACE)", pas.beginZScore)
	t.Logf("    longZScore  = %.2f (C++: LONG_PLACE)", pas.longZScore)
	t.Logf("    shortZScore = %.2f (C++: SHORT_PLACE)", pas.shortZScore)
	t.Logf("    maxPosition = %d", pas.maxPositionSize)

	if pas.beginZScore != 2.0 || pas.longZScore != 3.5 || pas.shortZScore != 0.5 {
		t.Fatal("动态阈值配置加载错误")
	}

	pas.Start()
	t.Log("  策略启动成功")

	// ========================================
	// Phase 2: 空仓状态 - 阈值应为 BEGIN_PLACE
	// ========================================
	t.Log("\n[Phase 2] 空仓状态测试")

	// 确保 firstStrat 存在并设置初始持仓
	if pas.firstStrat == nil {
		t.Fatal("firstStrat 未初始化")
	}
	pas.firstStrat.NetPosPass = 0
	pas.setDynamicThresholds()

	t.Logf("  持仓: NetPosPass = %d", pas.firstStrat.NetPosPass)
	t.Logf("  入场阈值: Bid=%.2f, Ask=%.2f", pas.entryZScoreBid, pas.entryZScoreAsk)
	t.Logf("  撤单阈值: Bid=%.2f, Ask=%.2f", pas.exitZScoreBid, pas.exitZScoreAsk)

	// C++: 空仓时 tholdBidPlace = tholdAskPlace = BEGIN_PLACE
	assertFloat(t, "空仓 entryZScoreBid", pas.entryZScoreBid, 2.0)
	assertFloat(t, "空仓 entryZScoreAsk", pas.entryZScoreAsk, 2.0)
	assertFloat(t, "空仓 exitZScoreBid", pas.exitZScoreBid, 0.5)
	assertFloat(t, "空仓 exitZScoreAsk", pas.exitZScoreAsk, 0.5)

	// ========================================
	// Phase 3: 模拟行情输入，建立价差统计
	// ========================================
	t.Log("\n[Phase 3] 行情输入与价差统计")

	// 模拟 100 个行情数据点，建立价差均值和标准差
	basePrice1 := 6000.0
	basePrice2 := 5990.0
	for i := 0; i < 100; i++ {
		// 添加随机波动
		noise1 := float64(i%10-5) * 0.5
		noise2 := float64(i%10-5) * 0.3

		md1 := &mdpb.MarketDataUpdate{
			Symbol:    "ag2603",
			Exchange:  "SHFE",
			Timestamp: uint64(time.Now().UnixNano()),
			BidPrice:  []float64{basePrice1 + noise1 - 0.5},
			BidQty:    []uint32{100},
			AskPrice:  []float64{basePrice1 + noise1 + 0.5},
			AskQty:    []uint32{100},
			LastPrice: basePrice1 + noise1,
		}

		md2 := &mdpb.MarketDataUpdate{
			Symbol:    "ag2605",
			Exchange:  "SHFE",
			Timestamp: uint64(time.Now().UnixNano()),
			BidPrice:  []float64{basePrice2 + noise2 - 0.5},
			BidQty:    []uint32{100},
			AskPrice:  []float64{basePrice2 + noise2 + 0.5},
			AskQty:    []uint32{100},
			LastPrice: basePrice2 + noise2,
		}

		pas.OnMarketData(md1)
		pas.OnMarketData(md2)
	}

	stats := pas.spreadAnalyzer.GetStats()
	t.Logf("  价差统计:")
	t.Logf("    Mean = %.4f", stats.Mean)
	t.Logf("    Std  = %.4f", stats.Std)
	t.Logf("    当前价差 = %.4f", pas.spreadAnalyzer.GetCurrentSpread())

	if stats.Std == 0 {
		t.Fatal("价差标准差为0，统计数据异常")
	}

	// ========================================
	// Phase 4: 模拟成交，建立多头持仓
	// ========================================
	t.Log("\n[Phase 4] 多头持仓状态测试")

	// 模拟逐步建立多头持仓
	testCases := []struct {
		netPosPass       int32
		expectedBidPlace float64
		expectedAskPlace float64
		description      string
	}{
		// C++: long_place_diff = 3.5 - 2.0 = 1.5
		// C++: short_place_diff = 2.0 - 0.5 = 1.5
		{0, 2.0, 2.0, "空仓"},
		{25, 2.375, 1.625, "25% 多头 (posRatio=0.25)"},
		{50, 2.75, 1.25, "50% 多头 (posRatio=0.5)"},
		{75, 3.125, 0.875, "75% 多头 (posRatio=0.75)"},
		{100, 3.5, 0.5, "满仓多头 (posRatio=1.0)"},
	}

	for _, tc := range testCases {
		pas.firstStrat.NetPosPass = tc.netPosPass
		pas.setDynamicThresholds()

		t.Logf("  [%s] NetPosPass=%d", tc.description, tc.netPosPass)
		t.Logf("    entryZScoreBid=%.3f (期望=%.3f)", pas.entryZScoreBid, tc.expectedBidPlace)
		t.Logf("    entryZScoreAsk=%.3f (期望=%.3f)", pas.entryZScoreAsk, tc.expectedAskPlace)

		assertFloatTolerance(t, fmt.Sprintf("%s Bid", tc.description), pas.entryZScoreBid, tc.expectedBidPlace, 0.001)
		assertFloatTolerance(t, fmt.Sprintf("%s Ask", tc.description), pas.entryZScoreAsk, tc.expectedAskPlace, 0.001)
	}

	// ========================================
	// Phase 5: 空头持仓状态测试
	// ========================================
	t.Log("\n[Phase 5] 空头持仓状态测试")

	shortTestCases := []struct {
		netPosPass       int32
		expectedBidPlace float64
		expectedAskPlace float64
		description      string
	}{
		// 空头时公式不同:
		// C++: tholdBidPlace = BEGIN + short_diff * netpos/maxPos
		// C++: tholdAskPlace = BEGIN - long_diff * netpos/maxPos
		{-25, 1.625, 2.375, "25% 空头 (posRatio=-0.25)"},
		{-50, 1.25, 2.75, "50% 空头 (posRatio=-0.5)"},
		{-75, 0.875, 3.125, "75% 空头 (posRatio=-0.75)"},
		{-100, 0.5, 3.5, "满仓空头 (posRatio=-1.0)"},
	}

	for _, tc := range shortTestCases {
		pas.firstStrat.NetPosPass = tc.netPosPass
		pas.setDynamicThresholds()

		t.Logf("  [%s] NetPosPass=%d", tc.description, tc.netPosPass)
		t.Logf("    entryZScoreBid=%.3f (期望=%.3f)", pas.entryZScoreBid, tc.expectedBidPlace)
		t.Logf("    entryZScoreAsk=%.3f (期望=%.3f)", pas.entryZScoreAsk, tc.expectedAskPlace)

		assertFloatTolerance(t, fmt.Sprintf("%s Bid", tc.description), pas.entryZScoreBid, tc.expectedBidPlace, 0.001)
		assertFloatTolerance(t, fmt.Sprintf("%s Ask", tc.description), pas.entryZScoreAsk, tc.expectedAskPlace, 0.001)
	}

	// ========================================
	// Phase 6: 撤单阈值动态调整测试
	// ========================================
	t.Log("\n[Phase 6] 撤单阈值动态调整测试")

	// 配置: exit_zscore=0.5 (BEGIN_REMOVE), long_exit_zscore=1.0, short_exit_zscore=0.2
	// long_remove_diff = 1.0 - 0.5 = 0.5
	// short_remove_diff = 0.5 - 0.2 = 0.3

	removeTestCases := []struct {
		netPosPass        int32
		expectedBidRemove float64
		expectedAskRemove float64
		description       string
	}{
		{0, 0.5, 0.5, "空仓"},
		{50, 0.75, 0.35, "50% 多头"},   // Bid: 0.5 + 0.5*0.5 = 0.75, Ask: 0.5 - 0.3*0.5 = 0.35
		{100, 1.0, 0.2, "满仓多头"},    // Bid: 0.5 + 0.5*1.0 = 1.0, Ask: 0.5 - 0.3*1.0 = 0.2
		{-50, 0.35, 0.75, "50% 空头"},  // Bid: 0.5 + 0.3*(-0.5) = 0.35, Ask: 0.5 - 0.5*(-0.5) = 0.75
		{-100, 0.2, 1.0, "满仓空头"},   // Bid: 0.5 + 0.3*(-1.0) = 0.2, Ask: 0.5 - 0.5*(-1.0) = 1.0
	}

	for _, tc := range removeTestCases {
		pas.firstStrat.NetPosPass = tc.netPosPass
		pas.setDynamicThresholds()

		t.Logf("  [%s] NetPosPass=%d", tc.description, tc.netPosPass)
		t.Logf("    exitZScoreBid=%.3f (期望=%.3f)", pas.exitZScoreBid, tc.expectedBidRemove)
		t.Logf("    exitZScoreAsk=%.3f (期望=%.3f)", pas.exitZScoreAsk, tc.expectedAskRemove)

		assertFloatTolerance(t, fmt.Sprintf("%s exitBid", tc.description), pas.exitZScoreBid, tc.expectedBidRemove, 0.001)
		assertFloatTolerance(t, fmt.Sprintf("%s exitAsk", tc.description), pas.exitZScoreAsk, tc.expectedAskRemove, 0.001)
	}

	// ========================================
	// Phase 7: 信号生成测试
	// ========================================
	t.Log("\n[Phase 7] 信号生成测试")

	// 重置为空仓
	pas.firstStrat.NetPosPass = 0
	pas.setDynamicThresholds()

	// 清空现有信号
	pas.GetSignals()

	// 发送极端行情，触发信号生成
	// 设置一个大的价差来触发信号
	extremeMD1 := &mdpb.MarketDataUpdate{
		Symbol:    "ag2603",
		Exchange:  "SHFE",
		Timestamp: uint64(time.Now().UnixNano()),
		BidPrice:  []float64{6020.0},
		BidQty:    []uint32{100},
		AskPrice:  []float64{6021.0},
		AskQty:    []uint32{100},
		LastPrice: 6020.5,
	}

	extremeMD2 := &mdpb.MarketDataUpdate{
		Symbol:    "ag2605",
		Exchange:  "SHFE",
		Timestamp: uint64(time.Now().UnixNano()),
		BidPrice:  []float64{5980.0},
		BidQty:    []uint32{100},
		AskPrice:  []float64{5981.0},
		AskQty:    []uint32{100},
		LastPrice: 5980.5,
	}

	pas.OnMarketData(extremeMD1)
	pas.OnMarketData(extremeMD2)

	signals := pas.GetSignals()
	t.Logf("  生成信号数量: %d", len(signals))

	for i, sig := range signals {
		t.Logf("    信号[%d]: Symbol=%s, Side=%v, Qty=%d, Price=%.2f",
			i, sig.Symbol, sig.Side, sig.Quantity, sig.Price)
	}

	// ========================================
	// Phase 8: 成交更新后阈值自动调整
	// ========================================
	t.Log("\n[Phase 8] 成交更新后阈值自动调整")

	// 模拟成交：买入 30 手
	pas.firstStrat.NetPosPass = 30
	pas.setDynamicThresholds()

	// posRatio = 30/100 = 0.3
	// expectedBidPlace = 2.0 + 1.5 * 0.3 = 2.45
	// expectedAskPlace = 2.0 - 1.5 * 0.3 = 1.55
	t.Logf("  成交后持仓: %d", pas.firstStrat.NetPosPass)
	t.Logf("  调整后阈值: Bid=%.3f, Ask=%.3f", pas.entryZScoreBid, pas.entryZScoreAsk)

	assertFloatTolerance(t, "成交后 Bid", pas.entryZScoreBid, 2.45, 0.001)
	assertFloatTolerance(t, "成交后 Ask", pas.entryZScoreAsk, 1.55, 0.001)

	// ========================================
	// Phase 9: 策略停止
	// ========================================
	t.Log("\n[Phase 9] 策略停止")

	pas.Stop()
	t.Log("  策略已停止")

	t.Log("\n========================================")
	t.Log("端到端测试完成 ✅")
	t.Log("========================================")
}

// TestDynamicThreshold_E2E_DisabledMode 测试禁用动态阈值模式
func TestDynamicThreshold_E2E_DisabledMode(t *testing.T) {
	t.Log("测试动态阈值禁用模式")

	pas := NewPairwiseArbStrategy("e2e_disabled_test")

	config := &StrategyConfig{
		StrategyID:      "e2e_disabled_test",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"ag2603", "ag2605"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"entry_zscore":          2.0,
			"exit_zscore":           0.5,
			"use_dynamic_threshold": false, // 禁用动态阈值
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	pas.Start()

	// 即使有持仓，阈值也应保持静态
	if pas.firstStrat != nil {
		pas.firstStrat.NetPosPass = 100 // 满仓
	}
	pas.setDynamicThresholds()

	t.Logf("  禁用模式下满仓阈值: Bid=%.2f, Ask=%.2f", pas.entryZScoreBid, pas.entryZScoreAsk)

	// 应该使用静态 entryZScore
	assertFloat(t, "禁用模式 Bid", pas.entryZScoreBid, 2.0)
	assertFloat(t, "禁用模式 Ask", pas.entryZScoreAsk, 2.0)

	pas.Stop()
	t.Log("  禁用模式测试通过 ✅")
}

// TestDynamicThreshold_E2E_ZeroMaxPosition 测试最大仓位为0的边界情况
func TestDynamicThreshold_E2E_ZeroMaxPosition(t *testing.T) {
	t.Log("测试最大仓位为0的边界情况")

	pas := NewPairwiseArbStrategy("e2e_zero_maxpos")

	config := &StrategyConfig{
		StrategyID:      "e2e_zero_maxpos",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"ag2603", "ag2605"},
		MaxPositionSize: 0, // 边界情况
		Parameters: map[string]interface{}{
			"entry_zscore":          2.0,
			"exit_zscore":           0.5,
			"use_dynamic_threshold": true,
			"begin_zscore":          2.0,
			"long_zscore":           3.5,
			"short_zscore":          0.5,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	pas.Start()

	// 最大仓位为0时，应该使用静态阈值（避免除零）
	pas.setDynamicThresholds()

	t.Logf("  maxPositionSize=0 时阈值: Bid=%.2f, Ask=%.2f", pas.entryZScoreBid, pas.entryZScoreAsk)

	// 应该安全处理，使用静态值
	assertFloat(t, "零仓位 Bid", pas.entryZScoreBid, 2.0)
	assertFloat(t, "零仓位 Ask", pas.entryZScoreAsk, 2.0)

	pas.Stop()
	t.Log("  零最大仓位测试通过 ✅")
}

// TestDynamicThreshold_E2E_CPPComparison C++ 对照测试
// 使用 C++ 计算示例进行精确验证
func TestDynamicThreshold_E2E_CPPComparison(t *testing.T) {
	t.Log("========================================")
	t.Log("C++ 对照测试 (SetThresholds.cpp)")
	t.Log("========================================")

	pas := NewPairwiseArbStrategy("cpp_comparison_unique_id")

	// 与 C++ 示例完全一致的配置
	// C++: BEGIN_PLACE=2.0, LONG_PLACE=3.5, SHORT_PLACE=0.5, maxPos=100
	config := &StrategyConfig{
		StrategyID:      "cpp_comparison_unique_id",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"ag2603", "ag2605"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"use_dynamic_threshold": true,
			"begin_zscore":          2.0,
			"long_zscore":           3.5,
			"short_zscore":          0.5,
			"entry_zscore":          2.0,
			"exit_zscore":           0.5,
			"max_position_size":     100.0, // 必须在 Parameters 中设置
		},
		Enabled: true,
	}

	pas.Initialize(config)
	
	// 重置 firstStrat.NetPosPass 为 0，避免从 daily_init 恢复的干扰
	if pas.firstStrat != nil {
		pas.firstStrat.NetPosPass = 0
		pas.firstStrat.NetPosPassYtd = 0
	}
	
	pas.Start()

	// C++ 计算示例 (来自 SetThresholds.cpp 注释):
	// long_place_diff = 3.5 - 2.0 = 1.5
	// short_place_diff = 2.0 - 0.5 = 1.5
	//
	// Case 1: netpos = 0 (空仓)
	//   tholdBid = 2.0, tholdAsk = 2.0
	//
	// Case 2: netpos = 100 (满仓多头)
	//   tholdBid = 2.0 + 1.5 * 1.0 = 3.5
	//   tholdAsk = 2.0 - 1.5 * 1.0 = 0.5
	//
	// Case 3: netpos = -100 (满仓空头)
	//   tholdBid = 2.0 + 1.5 * (-1.0) = 0.5
	//   tholdAsk = 2.0 - 1.5 * (-1.0) = 3.5
	//
	// Case 4: netpos = 50 (半仓多头)
	//   tholdBid = 2.0 + 1.5 * 0.5 = 2.75
	//   tholdAsk = 2.0 - 1.5 * 0.5 = 1.25

	cppTestCases := []struct {
		netpos      int32
		cppBidPlace float64
		cppAskPlace float64
		cppComment  string
	}{
		{0, 2.0, 2.0, "Case 1: netpos = 0 (空仓)"},
		{100, 3.5, 0.5, "Case 2: netpos = 100 (满仓多头, posRatio = 1.0)"},
		{-100, 0.5, 3.5, "Case 3: netpos = -100 (满仓空头, posRatio = -1.0)"},
		{50, 2.75, 1.25, "Case 4: netpos = 50 (半仓多头, posRatio = 0.5)"},
	}

	for _, tc := range cppTestCases {
		pas.firstStrat.NetPosPass = tc.netpos
		pas.setDynamicThresholds()

		t.Logf("\n  %s", tc.cppComment)
		t.Logf("    C++: tholdBid=%.2f, tholdAsk=%.2f", tc.cppBidPlace, tc.cppAskPlace)
		t.Logf("    Go:  entryBid=%.2f, entryAsk=%.2f", pas.entryZScoreBid, pas.entryZScoreAsk)

		if math.Abs(pas.entryZScoreBid-tc.cppBidPlace) > 0.001 {
			t.Errorf("    ❌ Bid 不匹配: Go=%.4f, C++=%.4f", pas.entryZScoreBid, tc.cppBidPlace)
		} else {
			t.Log("    ✅ Bid 匹配")
		}

		if math.Abs(pas.entryZScoreAsk-tc.cppAskPlace) > 0.001 {
			t.Errorf("    ❌ Ask 不匹配: Go=%.4f, C++=%.4f", pas.entryZScoreAsk, tc.cppAskPlace)
		} else {
			t.Log("    ✅ Ask 匹配")
		}
	}

	pas.Stop()
	t.Log("\n========================================")
	t.Log("C++ 对照测试完成")
	t.Log("========================================")
}

// TestDynamicThreshold_E2E_TopLevelMaxPositionSize 测试从顶层 MaxPositionSize 读取
// 修复: 实盘配置通常在顶层设置 max_position_size，而不是在 parameters 中
func TestDynamicThreshold_E2E_TopLevelMaxPositionSize(t *testing.T) {
	t.Log("测试从顶层 MaxPositionSize 读取配置")

	pas := NewPairwiseArbStrategy("top_level_maxpos_test")

	// 模拟实盘配置：max_position_size 在顶层，不在 parameters 中
	config := &StrategyConfig{
		StrategyID:      "top_level_maxpos_test",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"ag2603", "ag2605"},
		MaxPositionSize: 100, // 顶层设置
		Parameters: map[string]interface{}{
			"use_dynamic_threshold": true,
			"begin_zscore":          2.0,
			"long_zscore":           3.5,
			"short_zscore":          0.5,
			"entry_zscore":          2.0,
			"exit_zscore":           0.5,
			// 注意：这里故意不设置 max_position_size
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 验证 maxPositionSize 从顶层读取
	if pas.maxPositionSize != 100 {
		t.Errorf("maxPositionSize 应该从顶层读取: 期望 100, 实际 %d", pas.maxPositionSize)
	}

	t.Logf("  maxPositionSize = %d (从顶层 MaxPositionSize 读取)", pas.maxPositionSize)

	// 重置 firstStrat 并测试动态阈值
	if pas.firstStrat != nil {
		pas.firstStrat.NetPosPass = 0
	}
	
	pas.Start()

	// 测试满仓多头
	pas.firstStrat.NetPosPass = 100
	pas.setDynamicThresholds()

	// 如果 maxPositionSize 正确为 100，则 posRatio = 1.0
	// entryBid = 2.0 + 1.5 * 1.0 = 3.5
	// entryAsk = 2.0 - 1.5 * 1.0 = 0.5
	t.Logf("  满仓多头: entryBid=%.2f, entryAsk=%.2f", pas.entryZScoreBid, pas.entryZScoreAsk)

	if pas.entryZScoreBid != 3.5 {
		t.Errorf("entryZScoreBid 错误: 期望 3.5, 实际 %.2f (可能 maxPositionSize 未正确读取)", pas.entryZScoreBid)
	}
	if pas.entryZScoreAsk != 0.5 {
		t.Errorf("entryZScoreAsk 错误: 期望 0.5, 实际 %.2f (可能 maxPositionSize 未正确读取)", pas.entryZScoreAsk)
	}

	pas.Stop()
	t.Log("  顶层 MaxPositionSize 读取测试通过 ✅")
}

// ============================================================================
// 辅助函数
// ============================================================================

func assertFloat(t *testing.T, name string, actual, expected float64) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: 期望 %.4f, 实际 %.4f", name, expected, actual)
	}
}

func assertFloatTolerance(t *testing.T, name string, actual, expected, tolerance float64) {
	t.Helper()
	if math.Abs(actual-expected) > tolerance {
		t.Errorf("%s: 期望 %.4f (±%.4f), 实际 %.4f", name, expected, tolerance, actual)
	}
}
