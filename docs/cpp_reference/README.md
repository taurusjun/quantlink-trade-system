# C++ 参考代码目录

本目录保存从旧系统 (tbsrc) 迁移到 Golang 时需要参考的关键 C++ 代码片段。

## 目录结构

| 文件 | 对应 Go 实现 | 说明 |
|------|-------------|------|
| `SetThresholds.cpp` | `pairwise_arb_strategy.go:setDynamicThresholds()` | 动态阈值调整 |
| `SendAggressiveOrder.cpp` | `pairwise_arb_strategy.go:sendAggressiveOrder()` | 主动追单逻辑 |
| `ExecutionStrategy_TradeCallback.cpp` | `pairwise_arb_strategy.go:updateLeg1Position()` | 成交回调处理 |

## 使用规则

1. **迁移前**: 必须先在此目录找到或添加对应的 C++ 代码
2. **实现时**: Go 代码注释中必须引用此处的 C++ 代码
3. **测试时**: 测试用例的预期值应基于 C++ 代码的计算结果

## 参数映射表

### PairwiseArbStrategy 阈值参数

| C++ 参数 | C++ 变量 | Go 参数 | 配置字段 |
|---------|---------|--------|---------|
| `BEGIN_PLACE` | `m_thold_first->BEGIN_PLACE` | `beginZScore` | `begin_zscore` |
| `LONG_PLACE` | `m_thold_first->LONG_PLACE` | `longZScore` | `long_zscore` |
| `SHORT_PLACE` | `m_thold_first->SHORT_PLACE` | `shortZScore` | `short_zscore` |
| `BEGIN_REMOVE` | `m_thold_first->BEGIN_REMOVE` | `exitZScore` | `exit_zscore` |
| `SLOP` | `m_thold->SLOP` | `aggressiveSlopTicks` | `aggressive_slop_ticks` |

### 运行时变量

| C++ 变量 | Go 变量 | 说明 |
|---------|--------|------|
| `m_firstStrat->m_tholdBidPlace` | `entryZScoreBid` | 做多入场阈值 |
| `m_firstStrat->m_tholdAskPlace` | `entryZScoreAsk` | 做空入场阈值 |
| `m_firstStrat->m_netpos_pass` | `leg1Position` | 第一腿净持仓 |
| `m_firstStrat->m_tholdMaxPos` | `maxPositionSize` | 最大持仓 |

---

**最后更新**: 2026-02-08
