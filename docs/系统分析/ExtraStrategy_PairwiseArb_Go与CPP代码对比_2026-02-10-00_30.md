# Go 代码与 C++ 原代码对比分析

**文档日期**: 2026-02-10
**作者**: Claude
**版本**: v1.0
**相关模块**: golang/pkg/strategy/, tbsrc/Strategies/

---

## 概述

本文档对比分析 Go 代码（`pairwise_arb_strategy.go` 和 `extra_strategy.go`）与 C++ 原代码（`ExecutionStrategy.h`、`ExtraStrategy.h`、`PairwiseArbStrategy.h`）的差异，包括变量和方法的映射关系。

**对比文件**:
- Go: `golang/pkg/strategy/extra_strategy.go` (677 行)
- Go: `golang/pkg/strategy/pairwise_arb_strategy.go` (2028 行)
- C++: `tbsrc/Strategies/include/ExecutionStrategy.h` (579 行)
- C++: `tbsrc/Strategies/include/ExtraStrategy.h` (31 行)
- C++: `tbsrc/Strategies/include/PairwiseArbStrategy.h` (76 行)

---

## 1. ExtraStrategy 变量对比

### 1.1 持仓字段 (C++: ExecutionStrategy.h:111-114)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_netpos` | `NetPos` | int32 | ✅ 已实现 |
| `m_netpos_pass` | `NetPosPass` | int32 | ✅ 已实现 |
| `m_netpos_pass_ytd` | `NetPosPassYtd` | int32 | ✅ 已实现 |
| `m_netpos_agg` | `NetPosAgg` | int32 | ✅ 已实现 |

### 1.2 订单统计 (C++: ExecutionStrategy.h:123-137)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_buyOpenOrders` | `BuyOpenOrders` | int32 | ✅ 已实现 |
| `m_sellOpenOrders` | `SellOpenOrders` | int32 | ✅ 已实现 |
| `m_improveCount` | `ImproveCount` | int32 | ✅ 已实现 |
| `m_crossCount` | `CrossCount` | int32 | ✅ 已实现 |
| `m_tradeCount` | `TradeCount` | int32 | ✅ 已实现 |
| `m_rejectCount` | `RejectCount` | int32 | ✅ 已实现 |
| `m_orderCount` | `OrderCount` | int32 | ✅ 已实现 |
| `m_cancelCount` | `CancelCount` | int32 | ✅ 已实现 |
| `m_confirmCount` | `ConfirmCount` | int32 | ✅ 已实现 |
| `m_cancelconfirmCount` | - | int32 | ❌ 未实现 |
| `m_priceCount` | - | int32 | ❌ 未实现 |
| `m_deltaCount` | - | int32 | ❌ 未实现 |
| `m_lossCount` | - | int32 | ❌ 未实现 |
| `m_qtyCount` | - | int32 | ❌ 未实现 |

### 1.3 成交量统计 (C++: ExecutionStrategy.h:139-153)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_buyQty` | `BuyQty` | float64 | ✅ 已实现 |
| `m_sellQty` | `SellQty` | float64 | ✅ 已实现 |
| `m_buyTotalQty` | `BuyTotalQty` | float64 | ✅ 已实现 |
| `m_sellTotalQty` | `SellTotalQty` | float64 | ✅ 已实现 |
| `m_buyOpenQty` | `BuyOpenQty` | float64 | ✅ 已实现 |
| `m_sellOpenQty` | `SellOpenQty` | float64 | ✅ 已实现 |
| `m_buyTotalValue` | `BuyTotalValue` | float64 | ✅ 已实现 |
| `m_sellTotalValue` | `SellTotalValue` | float64 | ✅ 已实现 |
| `m_buyAvgPrice` | `BuyAvgPrice` | float64 | ✅ 已实现 |
| `m_sellAvgPrice` | `SellAvgPrice` | float64 | ✅ 已实现 |
| `m_buyExchTx` | - | float64 | ❌ 未实现 (交易所手续费) |
| `m_sellExchTx` | - | float64 | ❌ 未实现 (交易所手续费) |
| `m_buyValue` | - | float64 | ❌ 未实现 |
| `m_sellValue` | - | float64 | ❌ 未实现 |

### 1.4 PNL 字段 (C++: ExecutionStrategy.h:160-165)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_realisedPNL` | `RealisedPNL` | float64 | ✅ 已实现 |
| `m_unrealisedPNL` | `UnrealisedPNL` | float64 | ✅ 已实现 |
| `m_netPNL` | `NetPNL` | float64 | ✅ 已实现 |
| `m_grossPNL` | `GrossPNL` | float64 | ✅ 已实现 |
| `m_maxPNL` | `MaxPNL` | float64 | ✅ 已实现 |
| `m_drawdown` | `Drawdown` | float64 | ✅ 已实现 |

### 1.5 追单控制 (C++: ExecutionStrategy.h:289-294)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `buyAggCount` | `BuyAggCount` | float64 | ✅ 已实现 |
| `sellAggCount` | `SellAggCount` | float64 | ✅ 已实现 |
| `buyAggOrder` | `BuyAggOrder` | float64 | ✅ 已实现 |
| `sellAggOrder` | `SellAggOrder` | float64 | ✅ 已实现 |
| `last_agg_time` | `LastAggTime` | uint64 | ✅ 已实现 |
| `last_agg_side` | `LastAggSide` | TransactionType | ✅ 已实现 |

### 1.6 订单映射 (C++: ExecutionStrategy.h:257-264)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_ordMap` | `OrdMap` | map[uint32]*OrderStats | ✅ 已实现 |
| `m_bidMap` | `BidMap` | map[float64]*OrderStats | ✅ 已实现 |
| `m_askMap` | `AskMap` | map[float64]*OrderStats | ✅ 已实现 |
| `m_sweepordMap` | - | - | ❌ 未实现 (扫单映射) |
| `m_bidMapCache` | - | - | ❌ 未实现 (缓存) |
| `m_askMapCache` | - | - | ❌ 未实现 (缓存) |

### 1.7 阈值字段 (C++: ExecutionStrategy.h:186-199)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_tholdBidPlace` | `TholdBidPlace` | float64 | ✅ 已实现 |
| `m_tholdBidRemove` | `TholdBidRemove` | float64 | ✅ 已实现 |
| `m_tholdAskPlace` | `TholdAskPlace` | float64 | ✅ 已实现 |
| `m_tholdAskRemove` | `TholdAskRemove` | float64 | ✅ 已实现 |
| `m_tholdMaxPos` | `Thold.MaxSize` | int32 | ✅ 在 ThresholdSet 中 |
| `m_tholdSize` | `Thold.Size` | int32 | ✅ 在 ThresholdSet 中 |
| `m_tholdBidSize` | `Thold.BidSize` | int32 | ❌ 未实现 |
| `m_tholdBidMaxPos` | `Thold.BidMaxSize` | int32 | ⚠️ 部分实现 |
| `m_tholdAskSize` | `Thold.AskSize` | int32 | ❌ 未实现 |
| `m_tholdAskMaxPos` | `Thold.AskMaxSize` | int32 | ⚠️ 部分实现 |

### 1.8 状态标志 (C++: ExecutionStrategy.h:90-100)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_onExit` | `OnExit` | bool | ✅ 已实现 |
| `m_onCancel` | `OnCancel` | bool | ✅ 已实现 |
| `m_onFlat` | `OnFlat` | bool | ✅ 已实现 |
| `m_Active` | `Active` | bool | ✅ 已实现 |
| `m_onStopLoss` | `OnStopLoss` | bool | ✅ 已实现 |
| `m_aggFlat` | `AggFlat` | bool | ✅ 已实现 |
| `m_onMaxPx` | - | bool | ❌ 未实现 |
| `m_onNewsFlat` | - | bool | ❌ 未实现 |
| `m_onTimeSqOff` | - | bool | ❌ 未实现 |

### 1.9 时间戳字段 (C++: ExecutionStrategy.h:85-108)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_lastTradeTime` | `LastTradeTime` | uint64 | ✅ 已实现 |
| `m_lastOrderTime` | `LastOrderTime` | uint64 | ✅ 已实现 |
| `m_lastHBTS` | - | uint64 | ❌ 未实现 |
| `m_lastOrdTS` | - | uint64 | ❌ 未实现 |
| `m_lastDetailTS` | - | uint64 | ❌ 未实现 |
| `m_lastCancelRejectTime` | - | uint64 | ❌ 未实现 |
| `m_lastCancelRejectOrderID` | - | uint32 | ❌ 未实现 |

---

## 2. ExtraStrategy 方法对比

### 2.1 订单发送方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `SendBidOrder2()` | `SendBidOrder2()` | ✅ 已实现 | 发送买单 |
| `SendAskOrder2()` | `SendAskOrder2()` | ✅ 已实现 | 发送卖单 |
| `SendCancelOrder(uint32_t orderID)` | `SendCancelOrder()` | ✅ 已实现 | 按订单ID撤单 |
| `SendCancelOrder(price, side)` | `SendCancelOrderByPrice()` | ✅ 已实现 | 按价格撤单 |
| `SendNewOrder()` | - | ⚠️ | 逻辑在 SendBidOrder2/SendAskOrder2 中 |
| `SendModifyOrder()` | - | ❌ 未实现 | 改单功能 |
| `SendBidOrder()` | - | ❌ | 旧版本，使用 SendBidOrder2 替代 |
| `SendAskOrder()` | - | ❌ | 旧版本，使用 SendAskOrder2 替代 |
| `SendSweepOrder()` | - | ❌ 未实现 | 扫单功能 |

### 2.2 回调处理方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `ORSCallBack()` | `HandleOrderUpdate()` | ✅ 已实现 | 订单回调入口 |
| `ProcessTrade()` | `ProcessTrade()` | ✅ 已实现 | 处理成交 |
| `ProcessNewConfirm()` | `ProcessNewConfirm()` | ✅ 已实现 | 新单确认 |
| `ProcessCancelConfirm()` | `ProcessCancelConfirm()` | ✅ 已实现 | 撤单确认 |
| `ProcessNewReject()` | `ProcessNewReject()` | ✅ 已实现 | 新单拒绝 |
| `ProcessModifyConfirm()` | - | ❌ 未实现 | 改单确认 |
| `ProcessModifyReject()` | - | ❌ 未实现 | 改单拒绝 |
| `ProcessCancelReject()` | - | ❌ 未实现 | 撤单拒绝 |
| `ProcessSelfTrade()` | - | ❌ 未实现 | 自成交处理 |

### 2.3 阈值和风控方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `SetThresholds()` | `SetThresholds()` | ✅ 已实现 | 设置动态阈值 |
| `SetLinearThresholds()` | - | ❌ 未实现 | 线性阈值 |
| `HandleSquareoff()` | - | ❌ 未实现 | 平仓处理 |
| `HandleTimeLimitSquareoff()` | - | ❌ 未实现 | 时间限制平仓 |
| `CheckSquareoff()` | - | ❌ 未实现 | 平仓检查 |

### 2.4 其他方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `Reset()` | `Reset()` | ✅ 已实现 | 重置状态 |
| `CalculatePNL()` | - | ⚠️ | 在 PairwiseArbStrategy.updatePairwisePNL() 中 |
| `AddToOrderMap()` | `AddToOrderMap()` | ✅ 已实现 | 添加订单到映射 |
| `RemoveOrder()` | `RemoveFromOrderMap()` | ✅ 已实现 | 从映射移除订单 |
| `eraseFromOrderMap()` | - | ✅ | 逻辑在 RemoveFromOrderMap 中 |
| `GetOrderByID()` | `GetOrderByID()` | ✅ 已实现 | 按ID获取订单 |
| `GetOrderByPrice()` | `GetOrderByPrice()` | ✅ 已实现 | 按价格获取订单 |
| `MDCallBack()` | - | ⚠️ | 在 PairwiseArbStrategy.OnMarketData() 中 |
| `GetBidPrice()` | - | ❌ 未实现 | 获取买价 |
| `GetAskPrice()` | - | ❌ 未实现 | 获取卖价 |

---

## 3. PairwiseArbStrategy 变量对比

### 3.1 腿策略对象 (C++: PairwiseArbStrategy.h:63-66)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_firstStrat` | `firstStrat` | *ExtraStrategy | ✅ 已实现 |
| `m_secondStrat` | `secondStrat` | *ExtraStrategy | ✅ 已实现 |
| `m_thold_first` | `tholdFirst` | *ThresholdSet | ✅ 已实现 |
| `m_thold_second` | - | *ThresholdSet | ❌ 未实现 |
| `m_firstinstru` | `firstStrat.Instru` | *Instrument | ✅ 已实现 |
| `m_secondinstru` | `secondStrat.Instru` | *Instrument | ✅ 已实现 |

### 3.2 价格相关 (C++: PairwiseArbStrategy.h:44-55)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `i1_bestBid` | `bid1` | float64 | ✅ 已实现 |
| `i1_bestAsk` | `ask1` | float64 | ✅ 已实现 |
| `i2_bestBid` | `bid2` | float64 | ✅ 已实现 |
| `i2_bestAsk` | `ask2` | float64 | ✅ 已实现 |
| `avgSpreadRatio_ori` | `spreadAnalyzer.Mean` | float64 | ✅ 在 SpreadAnalyzer 中 |
| `avgSpreadRatio` | - | float64 | ⚠️ 通过 tValue 调整 |
| `currSpreadRatio` | `spreadAnalyzer.CurrentSpread` | float64 | ✅ 已实现 |
| `currSpreadRatio_prev` | - | float64 | ❌ 未实现 |
| `tValue` | `tValue` | float64 | ✅ 已实现 |
| `expectedRatio` | - | float64 | ❌ 未实现 |
| `iu` | - | float64 | ❌ 未实现 |
| `count` | - | float64 | ❌ 未实现 |

### 3.3 追单相关 (C++: PairwiseArbStrategy.h:60-62)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_netpos_agg1` | `firstStrat.NetPosAgg` | int32 | ✅ 已实现 |
| `m_netpos_agg2` | `secondStrat.NetPosAgg` | int32 | ✅ 已实现 |
| `m_agg_repeat` | `aggRepeat` | uint32 | ✅ 已实现 |
| `second_ordIDstart` | - | double | ❌ 未实现 |

### 3.4 订单映射 (C++: PairwiseArbStrategy.h:67-72)

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_bidMap1` | `firstStrat.BidMap` | PriceMap | ✅ 已实现 |
| `m_askMap1` | `firstStrat.AskMap` | PriceMap | ✅ 已实现 |
| `m_bidMap2` | `secondStrat.BidMap` | PriceMap | ✅ 已实现 |
| `m_askMap2` | `secondStrat.AskMap` | PriceMap | ✅ 已实现 |
| `m_ordMap1` | `firstStrat.OrdMap` | OrderMap* | ✅ 已实现 |
| `m_ordMap2` | `secondStrat.OrdMap` | OrderMap* | ✅ 已实现 |

### 3.5 风控字段

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `m_maxloss_limit` | - | double | ❌ 未实现 |
| `is_valid_mkdata` | - | bool | ❌ 未实现 |

### 3.6 矩阵数据

| C++ 字段 | Go 字段 | 类型 | 状态 |
|---------|--------|------|-----|
| `mx_daily_init` | - | map | ❌ 未实现 |

---

## 4. PairwiseArbStrategy 方法对比

### 4.1 核心方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `SendOrder()` | `generateSignals()` | ✅ 已实现 | 生成订单信号 |
| `SendAggressiveOrder()` | `sendAggressiveOrder()` | ✅ 已实现 | 主动追单 |
| `SetThresholds()` | `setDynamicThresholds()` | ✅ 已实现 | 动态阈值 |
| `CalcPendingNetposAgg()` | `calculatePendingNetpos()` | ✅ 已实现 | 计算待成交敞口 |
| `ORSCallBack()` | `OnOrderUpdate()` | ✅ 已实现 | 订单回调 |
| `MDCallBack()` | `OnMarketData()` | ✅ 已实现 | 行情回调 |
| `HandleSquareON()` | - | ❌ 未实现 | 开启平仓模式 |
| `HandleSquareoff()` | - | ❌ 未实现 | 平仓处理 |
| `HandlePassOrder()` | `updateLeg1Position()` | ✅ 已实现 | 被动单处理 |
| `HandleAggOrder()` | `updateLeg2Position()` | ✅ 已实现 | 主动单处理 |

### 4.2 价格计算方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `GetBidPrice_first()` | `optimizeOrderPrice()` | ⚠️ | 逻辑在 generateMultiLevelSignals 中 |
| `GetAskPrice_first()` | `optimizeOrderPrice()` | ⚠️ | 逻辑在 generateMultiLevelSignals 中 |
| `GetBidPrice_second()` | - | ⚠️ | 逻辑在 generateMultiLevelSignals 中 |
| `GetAskPrice_second()` | - | ⚠️ | 逻辑在 generateMultiLevelSignals 中 |

### 4.3 矩阵操作方法

| C++ 方法 | Go 方法 | 状态 | 说明 |
|---------|--------|------|------|
| `LoadMatrix()` | - | ❌ 未实现 | 加载矩阵 |
| `LoadMatrix2()` | - | ❌ 未实现 | 加载矩阵2 |
| `SaveMatrix()` | - | ❌ 未实现 | 保存矩阵 |
| `SaveMatrix2()` | - | ❌ 未实现 | 保存矩阵2 |
| `SendTCacheLeg1Pos()` | - | ❌ 未实现 | 发送缓存 |

---

## 5. 统计总结

### 5.1 ExtraStrategy 实现统计

| 分类 | 已实现 | 未实现 | 完成度 |
|------|--------|--------|--------|
| 持仓字段 | 4 | 0 | 100% |
| 订单统计 | 9 | 5 | 64% |
| 成交量统计 | 10 | 4 | 71% |
| PNL 字段 | 6 | 0 | 100% |
| 追单控制 | 6 | 0 | 100% |
| 订单映射 | 3 | 3 | 50% |
| 阈值字段 | 4 | 2 | 67% |
| 状态标志 | 6 | 3 | 67% |
| **总计** | **48** | **17** | **74%** |

### 5.2 ExtraStrategy 方法实现统计

| 分类 | 已实现 | 未实现 | 完成度 |
|------|--------|--------|--------|
| 订单发送 | 4 | 3 | 57% |
| 回调处理 | 5 | 4 | 56% |
| 阈值风控 | 1 | 4 | 20% |
| 其他方法 | 6 | 3 | 67% |
| **总计** | **16** | **14** | **53%** |

### 5.3 PairwiseArbStrategy 实现统计

| 分类 | 已实现 | 未实现 | 完成度 |
|------|--------|--------|--------|
| 腿策略对象 | 5 | 1 | 83% |
| 价格相关 | 7 | 4 | 64% |
| 追单相关 | 3 | 1 | 75% |
| 订单映射 | 6 | 0 | 100% |
| 风控字段 | 0 | 2 | 0% |
| 核心方法 | 8 | 2 | 80% |
| **总计** | **29** | **10** | **74%** |

---

## 6. 架构对比

### 6.1 C++ 架构

```
PairwiseArbStrategy : ExecutionStrategy
├── m_firstStrat  : ExtraStrategy* (ExecutionStrategy 子类)
│   ├── m_netpos_pass, m_netpos_agg
│   ├── m_ordMap, m_bidMap, m_askMap
│   └── m_thold
├── m_secondStrat : ExtraStrategy* (ExecutionStrategy 子类)
│   ├── m_netpos_pass, m_netpos_agg
│   ├── m_ordMap, m_bidMap, m_askMap
│   └── m_thold
├── m_thold_first : ThresholdSet*
├── m_thold_second : ThresholdSet*
└── m_ordMap1/m_ordMap2 → 指向 firstStrat/secondStrat.m_ordMap
```

### 6.2 Go 架构 (当前实现)

```
PairwiseArbStrategy
├── *BaseStrategy (嵌入)
├── firstStrat   : *ExtraStrategy ✅
│   ├── NetPosPass, NetPosAgg ✅
│   ├── OrdMap, BidMap, AskMap ✅
│   └── Thold : *ThresholdSet ✅
├── secondStrat  : *ExtraStrategy ✅
│   ├── NetPosPass, NetPosAgg ✅
│   ├── OrdMap, BidMap, AskMap ✅
│   └── Thold : *ThresholdSet (未独立)
├── tholdFirst   : *ThresholdSet ✅
└── 兼容字段: leg1Position, leg2Position (待废弃)
```

### 6.3 差异分析

| 方面 | C++ | Go | 差异 |
|------|-----|-----|------|
| 继承关系 | PairwiseArbStrategy : ExecutionStrategy | PairwiseArbStrategy 嵌入 *BaseStrategy | Go 使用组合代替继承 |
| 腿对象 | ExtraStrategy 继承 ExecutionStrategy | ExtraStrategy 独立结构体 | 结构相似，Go 更扁平 |
| 阈值配置 | 每条腿独立 thold | 共用 tholdFirst | **需要添加 tholdSecond** |
| 订单映射 | 指针引用 | 直接使用 ExtraStrategy 的 maps | 一致 |

---

## 7. 参考资料

- C++ ExecutionStrategy: `tbsrc/Strategies/include/ExecutionStrategy.h`
- C++ ExtraStrategy: `tbsrc/Strategies/include/ExtraStrategy.h`
- C++ PairwiseArbStrategy: `tbsrc/Strategies/include/PairwiseArbStrategy.h`
- Go ExtraStrategy: `golang/pkg/strategy/extra_strategy.go`
- Go PairwiseArbStrategy: `golang/pkg/strategy/pairwise_arb_strategy.go`

---

**最后更新**: 2026-02-10 00:30
