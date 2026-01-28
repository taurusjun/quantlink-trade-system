package strategy

import (
	"testing"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
)

func TestNewStrategyManager(t *testing.T) {
	sm := NewStrategyManager(nil)
	if sm == nil {
		t.Fatal("NewStrategyManager returned nil")
	}
	if sm.GetStrategyCount() != 0 {
		t.Errorf("Expected 0 strategies, got %d", sm.GetStrategyCount())
	}
}

func TestStrategyManagerLoadStrategies(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{
			ID:         "test_strategy_1",
			Type:       "passive",
			Enabled:    true,
			Symbols:    []string{"ag2502"},
			Allocation: 0.5,
			Parameters: map[string]interface{}{
				"spread_multiplier": 0.5,
			},
		},
		{
			ID:         "test_strategy_2",
			Type:       "pairwise_arb",
			Enabled:    true,
			Symbols:    []string{"ag2502", "ag2504"},
			Allocation: 0.5,
			Parameters: map[string]interface{}{
				"entry_zscore": 2.0,
			},
		},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	if sm.GetStrategyCount() != 2 {
		t.Errorf("Expected 2 strategies, got %d", sm.GetStrategyCount())
	}

	// 检查策略是否正确加载
	s1, ok := sm.GetStrategy("test_strategy_1")
	if !ok {
		t.Error("Strategy test_strategy_1 not found")
	}
	if s1.GetType() != "passive" {
		t.Errorf("Expected type 'passive', got '%s'", s1.GetType())
	}

	s2, ok := sm.GetStrategy("test_strategy_2")
	if !ok {
		t.Error("Strategy test_strategy_2 not found")
	}
	if s2.GetType() != "pairwise_arb" {
		t.Errorf("Expected type 'pairwise_arb', got '%s'", s2.GetType())
	}
}

func TestStrategyManagerDisabledStrategy(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{
			ID:      "enabled_strategy",
			Type:    "passive",
			Enabled: true,
			Symbols: []string{"ag2502"},
		},
		{
			ID:      "disabled_strategy",
			Type:    "passive",
			Enabled: false,
			Symbols: []string{"cu2503"},
		},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	// 只有启用的策略被加载
	if sm.GetStrategyCount() != 1 {
		t.Errorf("Expected 1 strategy, got %d", sm.GetStrategyCount())
	}

	_, ok := sm.GetStrategy("enabled_strategy")
	if !ok {
		t.Error("Enabled strategy not found")
	}

	_, ok = sm.GetStrategy("disabled_strategy")
	if ok {
		t.Error("Disabled strategy should not be loaded")
	}
}

func TestStrategyManagerAddRemoveStrategy(t *testing.T) {
	sm := NewStrategyManager(nil)

	// 动态添加策略
	cfg := config.StrategyItemConfig{
		ID:         "dynamic_strategy",
		Type:       "passive",
		Enabled:    true,
		Symbols:    []string{"ag2502"},
		Allocation: 0.3,
	}

	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("AddStrategy failed: %v", err)
	}

	if sm.GetStrategyCount() != 1 {
		t.Errorf("Expected 1 strategy, got %d", sm.GetStrategyCount())
	}

	// 动态移除策略
	err = sm.RemoveStrategy("dynamic_strategy")
	if err != nil {
		t.Fatalf("RemoveStrategy failed: %v", err)
	}

	if sm.GetStrategyCount() != 0 {
		t.Errorf("Expected 0 strategies, got %d", sm.GetStrategyCount())
	}

	// 移除不存在的策略
	err = sm.RemoveStrategy("non_existent")
	if err == nil {
		t.Error("Expected error when removing non-existent strategy")
	}
}

func TestStrategyManagerDuplicateStrategy(t *testing.T) {
	sm := NewStrategyManager(nil)

	cfg := config.StrategyItemConfig{
		ID:      "duplicate_test",
		Type:    "passive",
		Enabled: true,
		Symbols: []string{"ag2502"},
	}

	// 第一次添加
	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("First AddStrategy failed: %v", err)
	}

	// 第二次添加相同ID应该失败
	err = sm.AddStrategy(cfg)
	if err == nil {
		t.Error("Expected error when adding duplicate strategy")
	}
}

func TestStrategyManagerGetAllStrategies(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{ID: "s1", Type: "passive", Enabled: true, Symbols: []string{"ag2502"}},
		{ID: "s2", Type: "passive", Enabled: true, Symbols: []string{"cu2503"}},
		{ID: "s3", Type: "passive", Enabled: true, Symbols: []string{"au2504"}},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	all := sm.GetAllStrategies()
	if len(all) != 3 {
		t.Errorf("Expected 3 strategies, got %d", len(all))
	}

	// 验证返回的是副本，修改不影响原始数据
	delete(all, "s1")
	if sm.GetStrategyCount() != 3 {
		t.Error("GetAllStrategies should return a copy")
	}
}

func TestStrategyManagerGetStrategyIDs(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{ID: "alpha", Type: "passive", Enabled: true, Symbols: []string{"ag2502"}},
		{ID: "beta", Type: "passive", Enabled: true, Symbols: []string{"cu2503"}},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	ids := sm.GetStrategyIDs()
	if len(ids) != 2 {
		t.Errorf("Expected 2 IDs, got %d", len(ids))
	}

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	if !idMap["alpha"] || !idMap["beta"] {
		t.Error("Missing expected strategy IDs")
	}
}

func TestStrategyManagerActivateDeactivate(t *testing.T) {
	sm := NewStrategyManager(nil)

	cfg := config.StrategyItemConfig{
		ID:      "control_test",
		Type:    "passive",
		Enabled: true,
		Symbols: []string{"ag2502"},
	}

	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("AddStrategy failed: %v", err)
	}

	// 激活策略
	err = sm.ActivateStrategy("control_test")
	if err != nil {
		t.Fatalf("ActivateStrategy failed: %v", err)
	}

	// 停用策略
	err = sm.DeactivateStrategy("control_test")
	if err != nil {
		t.Fatalf("DeactivateStrategy failed: %v", err)
	}

	// 激活不存在的策略
	err = sm.ActivateStrategy("non_existent")
	if err == nil {
		t.Error("Expected error when activating non-existent strategy")
	}
}

func TestStrategyManagerGetStatus(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{ID: "s1", Type: "passive", Enabled: true, Symbols: []string{"ag2502"}, Allocation: 0.6},
		{ID: "s2", Type: "pairwise_arb", Enabled: true, Symbols: []string{"ag2502", "ag2504"}, Allocation: 0.4},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	status := sm.GetStatus()
	if status.TotalStrategies != 2 {
		t.Errorf("Expected 2 total strategies, got %d", status.TotalStrategies)
	}

	if len(status.StrategyStatuses) != 2 {
		t.Errorf("Expected 2 strategy statuses, got %d", len(status.StrategyStatuses))
	}

	// 检查分配比例
	if status.Allocations["s1"] != 0.6 {
		t.Errorf("Expected allocation 0.6 for s1, got %f", status.Allocations["s1"])
	}
}

func TestStrategyManagerGetStrategyStatus(t *testing.T) {
	sm := NewStrategyManager(nil)

	cfg := config.StrategyItemConfig{
		ID:         "status_test",
		Type:       "passive",
		Enabled:    true,
		Symbols:    []string{"ag2502", "cu2503"},
		Allocation: 0.75,
	}

	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("AddStrategy failed: %v", err)
	}

	info, err := sm.GetStrategyStatus("status_test")
	if err != nil {
		t.Fatalf("GetStrategyStatus failed: %v", err)
	}

	if info.ID != "status_test" {
		t.Errorf("Expected ID 'status_test', got '%s'", info.ID)
	}
	if info.Type != "passive" {
		t.Errorf("Expected type 'passive', got '%s'", info.Type)
	}
	if len(info.Symbols) != 2 {
		t.Errorf("Expected 2 symbols, got %d", len(info.Symbols))
	}
	if info.Allocation != 0.75 {
		t.Errorf("Expected allocation 0.75, got %f", info.Allocation)
	}

	// 获取不存在的策略状态
	_, err = sm.GetStrategyStatus("non_existent")
	if err == nil {
		t.Error("Expected error for non-existent strategy")
	}
}

func TestStrategyManagerSetAllocation(t *testing.T) {
	sm := NewStrategyManager(nil)

	cfg := config.StrategyItemConfig{
		ID:         "alloc_test",
		Type:       "passive",
		Enabled:    true,
		Symbols:    []string{"ag2502"},
		Allocation: 0.5,
	}

	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("AddStrategy failed: %v", err)
	}

	// 设置新的分配比例
	err = sm.SetAllocation("alloc_test", 0.8)
	if err != nil {
		t.Fatalf("SetAllocation failed: %v", err)
	}

	allocs := sm.GetAllocations()
	if allocs["alloc_test"] != 0.8 {
		t.Errorf("Expected allocation 0.8, got %f", allocs["alloc_test"])
	}

	// 设置无效的分配比例
	err = sm.SetAllocation("alloc_test", 1.5)
	if err == nil {
		t.Error("Expected error for allocation > 1")
	}

	err = sm.SetAllocation("alloc_test", -0.1)
	if err == nil {
		t.Error("Expected error for allocation < 0")
	}

	// 设置不存在策略的分配
	err = sm.SetAllocation("non_existent", 0.5)
	if err == nil {
		t.Error("Expected error for non-existent strategy")
	}
}

func TestStrategyManagerGetAggregatedPNL(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{ID: "pnl_s1", Type: "passive", Enabled: true, Symbols: []string{"ag2502"}},
		{ID: "pnl_s2", Type: "passive", Enabled: true, Symbols: []string{"cu2503"}},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	agg := sm.GetAggregatedPNL()
	if agg == nil {
		t.Fatal("GetAggregatedPNL returned nil")
	}

	// 初始PNL应该为0
	if agg.TotalPnL != 0 {
		t.Errorf("Expected total PNL 0, got %f", agg.TotalPnL)
	}

	if len(agg.ByStrategy) != 2 {
		t.Errorf("Expected 2 strategies in PNL breakdown, got %d", len(agg.ByStrategy))
	}
}

func TestStrategyManagerGetFirstStrategy(t *testing.T) {
	sm := NewStrategyManager(nil)

	// 空管理器
	first := sm.GetFirstStrategy()
	if first != nil {
		t.Error("Expected nil for empty manager")
	}

	// 添加策略后
	cfg := config.StrategyItemConfig{
		ID:      "first_test",
		Type:    "passive",
		Enabled: true,
		Symbols: []string{"ag2502"},
	}
	sm.AddStrategy(cfg)

	first = sm.GetFirstStrategy()
	if first == nil {
		t.Error("Expected non-nil strategy")
	}
}

func TestStrategyManagerStartStop(t *testing.T) {
	sm := NewStrategyManager(nil)

	cfg := config.StrategyItemConfig{
		ID:      "lifecycle_test",
		Type:    "passive",
		Enabled: true,
		Symbols: []string{"ag2502"},
	}

	err := sm.AddStrategy(cfg)
	if err != nil {
		t.Fatalf("AddStrategy failed: %v", err)
	}

	// 启动
	err = sm.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !sm.IsRunning() {
		t.Error("Expected manager to be running")
	}

	// 重复启动应该失败
	err = sm.Start()
	if err == nil {
		t.Error("Expected error for double start")
	}

	// 停止
	err = sm.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if sm.IsRunning() {
		t.Error("Expected manager to be stopped")
	}

	// 停止已停止的管理器应该成功（幂等）
	err = sm.Stop()
	if err != nil {
		t.Errorf("Stop on stopped manager should succeed: %v", err)
	}
}

func TestStrategyManagerForEach(t *testing.T) {
	sm := NewStrategyManager(nil)

	configs := []config.StrategyItemConfig{
		{ID: "fe_s1", Type: "passive", Enabled: true, Symbols: []string{"ag2502"}},
		{ID: "fe_s2", Type: "passive", Enabled: true, Symbols: []string{"cu2503"}},
		{ID: "fe_s3", Type: "passive", Enabled: true, Symbols: []string{"au2504"}},
	}

	err := sm.LoadStrategies(configs)
	if err != nil {
		t.Fatalf("LoadStrategies failed: %v", err)
	}

	count := 0
	ids := make(map[string]bool)

	sm.ForEach(func(id string, strategy Strategy) {
		count++
		ids[id] = true
	})

	if count != 3 {
		t.Errorf("Expected ForEach to iterate 3 times, got %d", count)
	}

	if !ids["fe_s1"] || !ids["fe_s2"] || !ids["fe_s3"] {
		t.Error("ForEach did not iterate all strategies")
	}
}
