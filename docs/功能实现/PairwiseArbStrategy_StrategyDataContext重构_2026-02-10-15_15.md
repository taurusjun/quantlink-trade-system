# PairwiseArbStrategy StrategyDataContext 重构

**文档日期**: 2026-02-10
**版本**: v1.0
**相关模块**: golang/pkg/strategy

---

## 概述

本次重构将 PairwiseArbStrategy 改为使用 `*StrategyDataContext` 嵌入，与其他策略（AggressiveStrategy、PassiveStrategy、HedgingStrategy）保持架构一致。

## 问题背景

### 重构前的不一致性

**其他策略** 使用统一模式：
```go
type AggressiveStrategy struct {
    *ExecutionStrategy   // C++: public ExecutionStrategy
    *StrategyDataContext // Go 特有字段
    // ... strategy-specific fields
}
```

**PairwiseArbStrategy** 直接定义重复字段：
```go
type PairwiseArbStrategy struct {
    *ExecutionStrategy // C++: public ExecutionStrategy
    // 没有 *StrategyDataContext！

    // 直接定义字段（与 StrategyDataContext 重复）:
    id                string
    strategyType      string
    config            *StrategyConfig
    pendingSignals    []*TradingSignal
    orders            map[string]*orspb.OrderUpdate
    // ... 等等
}
```

### 问题

1. **代码重复**: 手动定义了 `StrategyDataContext` 中已有的所有字段
2. **维护困难**: 添加新字段需要手动同步
3. **架构不一致**: 与其他策略模式不同
4. **缺少共享方法**: 无法使用 `StrategyDataContext` 的辅助方法

## 重构方案

### 结构体变更

```go
type PairwiseArbStrategy struct {
    *ExecutionStrategy   // C++: public ExecutionStrategy
    *StrategyDataContext // Go 特有字段（新增）

    // 腿策略对象
    firstStrat  *ExtraStrategy
    secondStrat *ExtraStrategy

    // 阈值配置
    tholdFirst  *ThresholdSet
    tholdSecond *ThresholdSet

    // ... 策略特有参数 ...

    // PairwiseArb 特有字段（保留）
    estimatedPosition *EstimatedPosition
    pnl               *PNL
    riskMetrics       *RiskMetrics
    running           bool

    mu sync.RWMutex
}
```

### 字段映射

| 原字段名 | 现在访问方式 | 说明 |
|---------|-------------|------|
| `pas.id` | `pas.ID` | 由 StrategyDataContext 提供 |
| `pas.strategyType` | `pas.Type` | 由 StrategyDataContext 提供 |
| `pas.config` | `pas.Config` | 由 StrategyDataContext 提供 |
| `pas.pendingSignals` | `pas.PendingSignals` | 由 StrategyDataContext 提供 |
| `pas.orders` | `pas.Orders` | 由 StrategyDataContext 提供 |
| `pas.status` | `pas.Status` | 由 StrategyDataContext 提供 |
| `pas.controlState` | `pas.ControlState` | 由 StrategyDataContext 提供 |
| `pas.privateIndicators` | `pas.PrivateIndicators` | 由 StrategyDataContext 提供 |
| `pas.lastMarketData` | `pas.LastMarketData` | 由 StrategyDataContext 提供 |

### 保留的特有字段

以下字段保留在 PairwiseArbStrategy 中，因为配对策略有特殊逻辑：

- `estimatedPosition` - 配对策略的特殊持仓计算（两腿分别计算）
- `pnl` - 盈亏统计
- `riskMetrics` - 风险指标
- `running` - 运行状态（有特殊控制逻辑）

### 构造函数变更

```go
func NewPairwiseArbStrategy(id string) *PairwiseArbStrategy {
    // 创建 ExecutionStrategy 基类
    baseExecStrategy := NewExecutionStrategy(strategyID, &Instrument{...})

    // 新增：创建 StrategyDataContext
    dataContext := NewStrategyDataContext(id, "pairwise_arb")
    dataContext.ControlState = NewStrategyControlState(false)

    pas := &PairwiseArbStrategy{
        ExecutionStrategy:   baseExecStrategy,
        StrategyDataContext: dataContext,  // 新增
        // ... 其他字段
    }
    return pas
}
```

## 架构一致性

重构后所有策略使用统一模式：

| 策略类型 | ExecutionStrategy | StrategyDataContext |
|---------|------------------|---------------------|
| AggressiveStrategy | ✅ | ✅ |
| PassiveStrategy | ✅ | ✅ |
| HedgingStrategy | ✅ | ✅ |
| PairwiseArbStrategy | ✅ | ✅ |

## 测试验证

- ✅ 所有策略单元测试通过（100+ 测试用例）
- ✅ 模拟器端到端测试通过
- ✅ 代码编译无错误

## 文件变更

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `golang/pkg/strategy/pairwise_arb_strategy.go` | 修改 | 添加 StrategyDataContext 嵌入，移除重复字段 |
| `golang/pkg/strategy/pairwise_arb_strategy_test.go` | 修改 | 更新字段访问方式 |

## 参考文档

- 策略基础接口: `golang/pkg/strategy/strategy.go`
- StrategyDataContext 定义: `golang/pkg/strategy/strategy.go:199-228`

---

**最后更新**: 2026-02-10 15:15
