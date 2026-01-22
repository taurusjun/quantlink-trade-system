# 单元测试修复：SharedIndicators 正确设置

**日期**: 2026-01-22
**任务**: 修复 TestPassiveStrategy_SignalGeneration 测试失败问题

---

## 问题分析

### 问题现象

测试 `TestPassiveStrategy_SignalGeneration` 失败：
```
--- FAIL: TestPassiveStrategy_SignalGeneration (0.75s)
    passive_strategy_test.go:107: Expected some signals to be generated
```

### 根本原因

在删除兼容性代码时，删除了 `PassiveStrategy.Initialize()` 中创建 "spread" 和 "order_imbalance" 指标的代码。

**问题链**:
1. PassiveStrategy 依赖 "spread" 和 "order_imbalance" 指标
2. 在生产环境中，这些指标由 StrategyEngine 通过 SharedIndicators 提供
3. 单元测试中没有 StrategyEngine，也没有设置 SharedIndicators
4. `generateSignals()` 调用 `GetIndicator("spread")` 失败，直接返回
5. 没有生成任何信号，测试失败

**代码流程**:
```go
// PassiveStrategy.generateSignals()
spread, ok := ps.GetIndicator("spread")
if !ok {
    return  // ❌ 找不到指标，直接返回
}
```

---

## 解决方案

### 原则

**不使用降级方案**，而是在单元测试中正确模拟 StrategyEngine 的行为：
1. 创建 SharedIndicators
2. 通过 `SetSharedIndicators()` 设置到策略
3. 在调用 `OnMarketData()` 前更新共享指标

这样可以确保测试环境与生产环境一致。

### 实施步骤

#### 1. 保持 PassiveStrategy.Initialize() 简洁

**文件**: `pkg/strategy/passive_strategy.go`

删除了降级方案，只保留私有指标的创建：
```go
func (ps *PassiveStrategy) Initialize(config *StrategyConfig) error {
    ps.Config = config
    // Load parameters...

    // Initialize private indicators (strategy-specific)
    ewmaConfig := map[string]interface{}{
        "period":      20.0,
        "max_history": 100.0,
    }
    _, err := ps.PrivateIndicators.Create("ewma_20", "ewma", ewmaConfig)
    if err != nil {
        return fmt.Errorf("failed to create EWMA indicator: %w", err)
    }

    // Note: Shared indicators (Spread, OrderImbalance, Volatility) MUST be
    // initialized by the StrategyEngine and attached via SetSharedIndicators().
    // In unit tests, they must be manually set up.
    // 注意：共享指标必须由 StrategyEngine 初始化并通过 SetSharedIndicators() 附加。
    // 在单元测试中，必须手动设置。

    ps.Status.StartTime = time.Now()
    return nil
}
```

**关键点**:
- ✅ 只创建策略私有的指标（如 EWMA）
- ✅ 不创建共享指标（Spread、OrderImbalance、Volatility）
- ✅ 添加注释说明共享指标的要求

#### 2. 创建测试辅助函数

**文件**: `pkg/strategy/passive_strategy_test.go`

添加 `setupSharedIndicators()` 辅助函数：
```go
// setupSharedIndicators creates shared indicators for testing
// This simulates what StrategyEngine does in production
func setupSharedIndicators(t *testing.T) *indicators.IndicatorLibrary {
    t.Helper()

    sharedInds := indicators.NewIndicatorLibrary()

    // Create Spread indicator
    spreadConfig := map[string]interface{}{
        "absolute":    true,
        "max_history": 100.0,
    }
    _, err := sharedInds.Create("spread", "spread", spreadConfig)
    if err != nil {
        t.Fatalf("Failed to create Spread indicator: %v", err)
    }

    // Create OrderImbalance indicator
    oiConfig := map[string]interface{}{
        "levels":        5.0,
        "volume_weight": true,
        "max_history":   100.0,
    }
    _, err = sharedInds.Create("order_imbalance", "order_imbalance", oiConfig)
    if err != nil {
        t.Fatalf("Failed to create OrderImbalance indicator: %v", err)
    }

    // Create Volatility indicator
    volConfig := map[string]interface{}{
        "window":          20.0,
        "use_log_returns": true,
        "max_history":     100.0,
    }
    _, err = sharedInds.Create("volatility", "volatility", volConfig)
    if err != nil {
        t.Fatalf("Failed to create Volatility indicator: %v", err)
    }

    return sharedInds
}
```

**关键点**:
- ✅ 使用 `t.Helper()` 标记为辅助函数
- ✅ 创建所有必需的共享指标
- ✅ 返回 `*indicators.IndicatorLibrary`

#### 3. 修改测试用例

**文件**: `pkg/strategy/passive_strategy_test.go`

在 `TestPassiveStrategy_SignalGeneration` 中添加共享指标设置：
```go
func TestPassiveStrategy_SignalGeneration(t *testing.T) {
    ps := NewPassiveStrategy("passive_1")

    config := &StrategyConfig{
        // ... config parameters
    }

    err := ps.Initialize(config)
    if err != nil {
        t.Fatalf("Failed to initialize: %v", err)
    }

    // Setup shared indicators (normally done by StrategyEngine)
    // 设置共享指标（通常由 StrategyEngine 完成）
    sharedIndicators := setupSharedIndicators(t)
    ps.SetSharedIndicators(sharedIndicators)

    ps.Start()

    // Feed market data
    for i := 0; i < 50; i++ {
        md := &mdpb.MarketDataUpdate{
            // ... market data
        }
        // Update shared indicators first (simulating StrategyEngine behavior)
        // 先更新共享指标（模拟 StrategyEngine 的行为）
        sharedIndicators.UpdateAll(md)
        ps.OnMarketData(md)
        time.Sleep(10 * time.Millisecond)
    }

    // ... rest of test
}
```

**关键点**:
1. ✅ 调用 `setupSharedIndicators(t)` 创建共享指标
2. ✅ 调用 `ps.SetSharedIndicators(sharedIndicators)` 设置
3. ✅ 在每次 `OnMarketData()` 前调用 `sharedIndicators.UpdateAll(md)`

---

## 修改文件清单

| 文件 | 修改类型 | 说明 |
|------|---------|------|
| `pkg/strategy/passive_strategy.go` | 简化 | 移除降级方案，添加注释 |
| `pkg/strategy/passive_strategy_test.go` | 增强 | 添加 setupSharedIndicators() 函数 |
| `pkg/strategy/passive_strategy_test.go` | 修改 | 在测试中正确设置 SharedIndicators |

---

## 测试结果

### 修复前
```
--- FAIL: TestPassiveStrategy_SignalGeneration (0.75s)
    passive_strategy_test.go:107: Expected some signals to be generated
FAIL
```

### 修复后
```
=== RUN   TestPassiveStrategy_SignalGeneration
--- PASS: TestPassiveStrategy_SignalGeneration (0.75s)
PASS
```

### 全部测试
```bash
$ go test ./pkg/strategy/... -v
...
PASS
ok  	github.com/yourusername/quantlink-trade-system/pkg/strategy	2.848s
```

**通过率**: ✅ **100% (37/37)**

---

## 架构改进

### 测试与生产环境一致性

**之前（降级方案）**:
```
单元测试环境:
  PassiveStrategy → PrivateIndicators (包含 spread, order_imbalance)
                     ↓ 降级
                     (未设置 SharedIndicators)

生产环境:
  StrategyEngine → SharedIndicators (spread, order_imbalance)
                    ↓
  PassiveStrategy → 使用 SharedIndicators
```
❌ 问题：测试和生产行为不一致

**修复后（正确方案）**:
```
单元测试环境:
  Test Helper → setupSharedIndicators()
                 ↓
  PassiveStrategy.SetSharedIndicators()
                 ↓
  Test Loop → sharedIndicators.UpdateAll(md)
                 ↓
  PassiveStrategy.OnMarketData(md)

生产环境:
  StrategyEngine → SharedIndicatorPool
                    ↓
  Strategy.SetSharedIndicators()
                    ↓
  Engine Loop → sharedIndPool.UpdateAll(symbol, md)
                 ↓
  Strategy.OnMarketData(md)
```
✅ 优势：测试和生产行为完全一致

### 职责分离

**PassiveStrategy 职责**:
- ✅ 创建策略私有指标（EWMA 等）
- ✅ 使用共享指标（通过 GetIndicator）
- ❌ **不**创建共享指标

**StrategyEngine 职责**:
- ✅ 创建和管理共享指标
- ✅ 更新共享指标
- ✅ 分发共享指标到各策略

**单元测试职责**:
- ✅ 模拟 StrategyEngine 的行为
- ✅ 正确设置和更新共享指标

---

## 最佳实践

### 1. 单元测试中设置 SharedIndicators

对于需要共享指标的策略测试，始终：
```go
// Step 1: Initialize strategy
strategy.Initialize(config)

// Step 2: Setup shared indicators (simulating StrategyEngine)
sharedIndicators := setupSharedIndicators(t)
strategy.SetSharedIndicators(sharedIndicators)

// Step 3: Start strategy
strategy.Start()

// Step 4: Feed market data with indicator updates
for _, md := range marketDataList {
    sharedIndicators.UpdateAll(md)  // Update first!
    strategy.OnMarketData(md)
}
```

### 2. 创建测试辅助函数

创建可重用的辅助函数：
```go
func setupSharedIndicators(t *testing.T) *indicators.IndicatorLibrary {
    t.Helper()  // Important!

    sharedInds := indicators.NewIndicatorLibrary()

    // Create all required shared indicators
    // ...

    return sharedInds
}
```

### 3. 明确指标职责

在策略代码中添加注释说明：
```go
// Note: Shared indicators MUST be initialized by the StrategyEngine
// and attached via SetSharedIndicators(). In unit tests, they must
// be manually set up.
```

### 4. 不使用降级方案

避免在策略初始化中创建共享指标作为"降级"：
- ❌ 不利于职责分离
- ❌ 测试与生产行为不一致
- ❌ 可能导致重复计算（如果同时存在共享和私有版本）

---

## 其他策略的适用性

这个修复方法适用于所有使用共享指标的策略测试：

### AggressiveStrategy
使用私有指标，不需要修改（已通过）

### HedgingStrategy
使用私有指标，不需要修改（已通过）

### PairwiseArbStrategy
使用私有指标，不需要修改（已通过）

### PassiveStrategy
✅ 已修复，使用共享指标

**总结**: 只有 PassiveStrategy 明确依赖共享指标，其他策略都使用私有指标，因此不受影响。

---

## 总结

### 修复内容

1. ✅ 删除了 PassiveStrategy 中的降级指标创建代码
2. ✅ 添加了测试辅助函数 `setupSharedIndicators()`
3. ✅ 修改了测试用例，正确设置和更新 SharedIndicators
4. ✅ 确保测试环境与生产环境行为一致

### 测试状态

- **修复前**: 36/37 通过 (97.3%)
- **修复后**: 37/37 通过 (100%) ✅

### 代码质量

1. ✅ **职责分离**: 策略不再创建共享指标
2. ✅ **测试一致性**: 测试完全模拟生产环境
3. ✅ **可维护性**: 测试辅助函数可重用
4. ✅ **文档完善**: 添加了清晰的注释说明

### 架构改进

- ✅ 更清晰的职责划分（Strategy vs Engine）
- ✅ 更一致的测试环境（与生产对齐）
- ✅ 更好的代码组织（测试辅助函数）

---

**状态**: ✅ **完成**
**测试通过率**: ✅ **100%**
**代码质量**: ✅ **高**
