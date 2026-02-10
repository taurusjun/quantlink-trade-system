# BaseStrategy vs ExecutionStrategy 对比与修复计划

**文档日期**: 2026-02-10
**版本**: v1.1
**最后更新**: 2026-02-10 10:50

---

## 修复进度

### ✅ 已完成

| 任务 | 说明 | 完成时间 |
|-----|------|---------|
| P0-1 撤单请求集成 | 添加 `GetPendingCancelOrders()`, `MarkCancelSent()`, `CancelAllActiveOrders()` 到 **BaseStrategy** | 2026-02-10 |
| P0-2 撤单拒绝回调 | `ProcessCancelReject()` 已集成到 `ProcessCancelRequests()` | 2026-02-10 |
| P1-1 风控变量 | 添加 `RmsQty`, `MaxOrderCount`, `MaxPosSize` 等风控变量 | 2026-02-10 |
| P1-2 时间控制变量 | 添加 `EndTimeH`, `EndTimeM`, `EndTimeEpoch` 等时间控制 | 2026-02-10 |
| P1-3 阈值控制变量 | 添加 `TholdBidSize`, `TholdAskSize`, `TholdMaxPos` 等阈值控制 | 2026-02-10 |
| P2-1 价格跟踪变量 | 添加 `Ltp`, `CurrPrice`, `TargetPrice`, `TheoBid`, `TheoAsk` 等 | 2026-02-10 |
| P2-2 时间戳变量 | 添加 `LastPosTS`, `LastFlatTS`, `ExchTS`, `LocalTS` 等时间戳 | 2026-02-10 |
| P2-3 撤单状态变量 | 添加 `PendingBidCancel`, `PendingAskCancel`, `CheckCancelQuantity` | 2026-02-10 |
| P2-4 订单状态变量 | 添加 `QuoteChanged`, `IsBidOrderCrossing`, `IsAskOrderCrossing` 等 | 2026-02-10 |
| Engine 集成 | 添加 `ProcessCancelRequests()` 到 timerLoop | 2026-02-10 |
| **架构简化** | 移除 `BaseStrategyAccessor` 和 `ExtraStrategyAccessor` 接口，直接在 `Strategy` 接口添加 `GetBaseStrategy()` | 2026-02-10 |

### 🔄 进行中

| 任务 | 说明 | 备注 |
|-----|------|------|
| - | - | - |

### ⏳ 待完成

| 任务 | 说明 | 优先级 |
|-----|------|-------|
| 调试方法 | `DumpOurBook()`, `DumpMktBook()` 等 | P3 |
| 监控方法 | `SendMonitorStrat*` 系列 | P3 |

---

## 概述

本文档对比 C++ 原代码 `ExecutionStrategy` 与 Go 新代码 `BaseStrategy`/`ExtraStrategy` 的变量和方法，分析缺失项并制定修复计划。

**原代码位置**: `/Users/user/PWorks/RD/tbsrc/Strategies/include/ExecutionStrategy.h`
**新代码位置**:
- `/Users/user/PWorks/RD/quantlink-trade-system/golang/pkg/strategy/strategy.go` (BaseStrategy)
- `/Users/user/PWorks/RD/quantlink-trade-system/golang/pkg/strategy/extra_strategy.go` (ExtraStrategy)

---

## 架构对比

### C++ 架构
```
ExecutionStrategy (基类)
    ├── 持仓管理 (m_netpos, m_netpos_pass, m_netpos_agg, m_netpos_pass_ytd)
    ├── 订单管理 (m_ordMap, m_bidMap, m_askMap, m_sweepordMap)
    ├── 阈值管理 (m_thold, m_tholdBidPlace, m_tholdAskPlace, ...)
    ├── 统计信息 (m_tradeCount, m_rejectCount, m_orderCount, ...)
    ├── PNL 计算 (m_realisedPNL, m_unrealisedPNL, m_netPNL, ...)
    └── 订单回调 (ORSCallBack, MDCallBack, AuctionCallBack)
```

### Go 架构
```
Strategy (接口)
    ├── GetBaseStrategy() *BaseStrategy   // 访问基类

BaseStrategy (基类 - 策略框架，对应 C++ ExecutionStrategy 基础部分)
    ├── 基础字段 (ID, Type, Config, Status)
    ├── 指标管理 (SharedIndicators, PrivateIndicators)
    ├── 持仓估算 (EstimatedPosition)
    ├── 信号管理 (PendingSignals)
    ├── 订单缓存 (Orders - 仅用于 UI 显示)
    └── 撤单管理 (CancelRequest, GetPendingCancelOrders, MarkCancelSent, CancelAllActiveOrders)

ExtraStrategy (执行策略 - 对应 C++ ExecutionStrategy/ExtraStrategy 扩展部分)
    ├── 持仓管理 (NetPos, NetPosPass, NetPosAgg, NetPosPassYtd)
    ├── 订单管理 (OrdMap, BidMap, AskMap, SweepOrdMap)
    ├── 阈值管理 (Thold, TholdBidPlace, TholdAskPlace, ...)
    ├── 统计信息 (TradeCount, RejectCount, OrderCount, ...)
    └── PNL 计算 (RealisedPNL, UnrealisedPNL, NetPNL, ...)
```

**结论**: Go 架构将 C++ `ExecutionStrategy` 拆分为两层：
- `BaseStrategy`: 策略框架层（生命周期、信号、指标、**撤单管理**）
- `ExtraStrategy`: 执行层（持仓、订单、阈值）

**架构简化说明** (2026-02-10):
- 移除了 `BaseStrategyAccessor` 接口，改为在 `Strategy` 接口直接添加 `GetBaseStrategy()` 方法
- 移除了 `ExtraStrategyAccessor` 接口
- Go 可以直接通过嵌入类型访问基类，无需额外的 Accessor 接口
- 撤单相关方法统一放在 `BaseStrategy` 中，与 C++ `ExecutionStrategy::SendCancelOrder` 对应

---

## 变量对比

### 1. 持仓相关变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_netpos` | int32_t | `NetPos` | ExtraStrategy | ✅ 已有 |
| `m_netpos_pass` | int32_t | `NetPosPass` | ExtraStrategy | ✅ 已有 |
| `m_netpos_pass_ytd` | int32_t | `NetPosPassYtd` | ExtraStrategy | ✅ 已有 |
| `m_netpos_agg` | int32_t | `NetPosAgg` | ExtraStrategy | ✅ 已有 |
| `m_rmsQty` | int32_t | - | - | ❌ 缺失 |

### 2. 订单统计变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_buyOpenOrders` | int32_t | `BuyOpenOrders` | ExtraStrategy | ✅ 已有 |
| `m_sellOpenOrders` | int32_t | `SellOpenOrders` | ExtraStrategy | ✅ 已有 |
| `m_improveCount` | int32_t | `ImproveCount` | ExtraStrategy | ✅ 已有 |
| `m_crossCount` | int32_t | `CrossCount` | ExtraStrategy | ✅ 已有 |
| `m_tradeCount` | int32_t | `TradeCount` | ExtraStrategy | ✅ 已有 |
| `m_rejectCount` | int32_t | `RejectCount` | ExtraStrategy | ✅ 已有 |
| `m_orderCount` | int32_t | `OrderCount` | ExtraStrategy | ✅ 已有 |
| `m_cancelCount` | int32_t | `CancelCount` | ExtraStrategy | ✅ 已有 |
| `m_confirmCount` | int32_t | `ConfirmCount` | ExtraStrategy | ✅ 已有 |
| `m_cancelconfirmCount` | int32_t | `CancelConfirmCount` | ExtraStrategy | ✅ 已有 |
| `m_priceCount` | int32_t | `PriceCount` | ExtraStrategy | ✅ 已有 |
| `m_deltaCount` | int32_t | `DeltaCount` | ExtraStrategy | ✅ 已有 |
| `m_lossCount` | int32_t | `LossCount` | ExtraStrategy | ✅ 已有 |
| `m_qtyCount` | int32_t | `QtyCount` | ExtraStrategy | ✅ 已有 |
| `m_maxOrderCount` | uint64_t | - | - | ❌ 缺失 |
| `m_maxPosSize` | uint64_t | - | - | ❌ 缺失 |

### 3. 成交量统计变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_buyQty` | double | `BuyQty` | ExtraStrategy | ✅ 已有 |
| `m_sellQty` | double | `SellQty` | ExtraStrategy | ✅ 已有 |
| `m_buyTotalQty` | double | `BuyTotalQty` | ExtraStrategy | ✅ 已有 |
| `m_sellTotalQty` | double | `SellTotalQty` | ExtraStrategy | ✅ 已有 |
| `m_buyOpenQty` | double | `BuyOpenQty` | ExtraStrategy | ✅ 已有 |
| `m_sellOpenQty` | double | `SellOpenQty` | ExtraStrategy | ✅ 已有 |
| `m_buyTotalValue` | double | `BuyTotalValue` | ExtraStrategy | ✅ 已有 |
| `m_sellTotalValue` | double | `SellTotalValue` | ExtraStrategy | ✅ 已有 |
| `m_buyAvgPrice` | double | `BuyAvgPrice` | ExtraStrategy | ✅ 已有 |
| `m_sellAvgPrice` | double | `SellAvgPrice` | ExtraStrategy | ✅ 已有 |
| `m_buyExchTx` | double | `BuyExchTx` | ExtraStrategy | ✅ 已有 |
| `m_sellExchTx` | double | `SellExchTx` | ExtraStrategy | ✅ 已有 |
| `m_buyValue` | double | `BuyValue` | ExtraStrategy | ✅ 已有 |
| `m_sellValue` | double | `SellValue` | ExtraStrategy | ✅ 已有 |
| `m_buyExchContractTx` | double | - | - | ❌ 缺失 |
| `m_sellExchContractTx` | double | - | - | ❌ 缺失 |
| `m_transTotalValue` | double | - | - | ❌ 缺失 |
| `m_transValue` | double | - | - | ❌ 缺失 |
| `m_maxTradedQty` | double | - | - | ❌ 缺失 |
| `m_buyPrice` | double | - | - | ❌ 缺失 |
| `m_sellPrice` | double | - | - | ❌ 缺失 |
| `m_avgQty` | double | - | - | ❌ 缺失 |

### 4. PNL 变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_realisedPNL` | double | `RealisedPNL` | ExtraStrategy | ✅ 已有 |
| `m_unrealisedPNL` | double | `UnrealisedPNL` | ExtraStrategy | ✅ 已有 |
| `m_netPNL` | double | `NetPNL` | ExtraStrategy | ✅ 已有 |
| `m_grossPNL` | double | `GrossPNL` | ExtraStrategy | ✅ 已有 |
| `m_maxPNL` | double | `MaxPNL` | ExtraStrategy | ✅ 已有 |
| `m_drawdown` | double | `Drawdown` | ExtraStrategy | ✅ 已有 |

### 5. 状态标志变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_onExit` | bool | `OnExit` | ExtraStrategy | ✅ 已有 |
| `m_onCancel` | bool | `OnCancel` | ExtraStrategy | ✅ 已有 |
| `m_onFlat` | bool | `OnFlat` | ExtraStrategy | ✅ 已有 |
| `m_Active` | bool | `Active` | ExtraStrategy | ✅ 已有 |
| `m_onStopLoss` | bool | `OnStopLoss` | ExtraStrategy | ✅ 已有 |
| `m_aggFlat` | bool | `AggFlat` | ExtraStrategy | ✅ 已有 |
| `m_onMaxPx` | bool | `OnMaxPx` | ExtraStrategy | ✅ 已有 |
| `m_onNewsFlat` | bool | `OnNewsFlat` | ExtraStrategy | ✅ 已有 |
| `m_onTimeSqOff` | bool | `OnTimeSqOff` | ExtraStrategy | ✅ 已有 |
| `m_sendMail` | bool | - | - | ❌ 缺失 |
| `m_optionStrategy` | bool | - | - | ❌ 缺失 |
| `callSquareOff` | bool | - | - | ❌ 缺失 |
| `quoteChanged` | bool | - | - | ❌ 缺失 |
| `isBidOrderCrossing` | bool | - | - | ❌ 缺失 |
| `isAskOrderCrossing` | bool | - | - | ❌ 缺失 |
| `isHedging` | bool | - | - | ❌ 缺失 |
| `hedgingSide` | bool | - | - | ❌ 缺失 |
| `m_lastTradeSide` | bool | - | - | ❌ 缺失 |
| `m_lastTrade` | bool | - | - | ❌ 缺失 |
| `m_pendingBidCancel` | bool | - | - | ❌ 缺失 |
| `m_pendingAskCancel` | bool | - | - | ❌ 缺失 |
| `m_checkCancelQuantity` | bool | - | - | ❌ 缺失 |
| `m_useNewsHandler` | bool | - | - | ❌ 缺失 |

### 6. 时间戳变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_lastHBTS` | uint64_t | `LastHBTS` | ExtraStrategy | ✅ 已有 |
| `m_lastOrdTS` | uint64_t | `LastOrdTS` | ExtraStrategy | ✅ 已有 |
| `m_lastDetailTS` | uint64_t | `LastDetailTS` | ExtraStrategy | ✅ 已有 |
| `m_lastTradeTime` | uint64_t | `LastTradeTime` | ExtraStrategy | ✅ 已有 |
| `m_lastOrderTime` | uint64_t | `LastOrderTime` | ExtraStrategy | ✅ 已有 |
| `m_lastCancelRejectTime` | uint64_t | `LastCancelRejectTime` | ExtraStrategy | ✅ 已有 |
| `m_lastCancelRejectOrderID` | uint32_t | `LastCancelRejectOrderID` | ExtraStrategy | ✅ 已有 |
| `m_lastPosTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastStsTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastFlatTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastPxTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastDeltaTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastLossTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastQtyTS` | uint64_t | - | - | ❌ 缺失 |
| `m_exchTS` | uint64_t | - | - | ❌ 缺失 |
| `m_localTS` | uint64_t | - | - | ❌ 缺失 |
| `m_lastSweepTradeTime` | uint64_t | - | - | ❌ 缺失 |

### 7. 订单映射变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_ordMap` | OrderMap | `OrdMap` | ExtraStrategy | ✅ 已有 |
| `m_bidMap` | PriceMap | `BidMap` | ExtraStrategy | ✅ 已有 |
| `m_askMap` | PriceMap | `AskMap` | ExtraStrategy | ✅ 已有 |
| `m_sweepordMap` | OrderMap | `SweepOrdMap` | ExtraStrategy | ✅ 已有 |
| `m_bidMapCache` | PriceMap | `BidMapCache` | ExtraStrategy | ✅ 已有 |
| `m_askMapCache` | PriceMap | `AskMapCache` | ExtraStrategy | ✅ 已有 |
| `m_bidMapCacheDel` | PriceMap | - | - | ❌ 缺失 |
| `m_askMapCacheDel` | PriceMap | - | - | ❌ 缺失 |

### 8. 阈值变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_thold` | ThresholdSet* | `Thold` | ExtraStrategy | ✅ 已有 |
| `m_tholdBidPlace` | double | `TholdBidPlace` | ExtraStrategy | ✅ 已有 |
| `m_tholdBidRemove` | double | `TholdBidRemove` | ExtraStrategy | ✅ 已有 |
| `m_tholdAskPlace` | double | `TholdAskPlace` | ExtraStrategy | ✅ 已有 |
| `m_tholdAskRemove` | double | `TholdAskRemove` | ExtraStrategy | ✅ 已有 |
| `m_tholdMaxPos` | int32_t | - | - | ❌ 缺失 |
| `m_tholdBeginPos` | int32_t | - | - | ❌ 缺失 |
| `m_tholdInc` | int32_t | - | - | ❌ 缺失 |
| `m_tholdSize` | int32_t | - | - | ❌ 缺失 |
| `m_tholdBidSize` | int32_t | - | - | ❌ 缺失 |
| `m_tholdBidMaxPos` | int32_t | - | - | ❌ 缺失 |
| `m_tholdAskSize` | int32_t | - | - | ❌ 缺失 |
| `m_tholdAskMaxPos` | int32_t | - | - | ❌ 缺失 |

### 9. 追单控制变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `buyAggCount` | double | `BuyAggCount` | ExtraStrategy | ✅ 已有 |
| `sellAggCount` | double | `SellAggCount` | ExtraStrategy | ✅ 已有 |
| `buyAggOrder` | double | `BuyAggOrder` | ExtraStrategy | ✅ 已有 |
| `sellAggOrder` | double | `SellAggOrder` | ExtraStrategy | ✅ 已有 |
| `last_agg_time` | uint64_t | `LastAggTime` | ExtraStrategy | ✅ 已有 |
| `last_agg_side` | TransactionType | `LastAggSide` | ExtraStrategy | ✅ 已有 |

### 10. 时间控制变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_endTimeH` | int32_t | - | - | ❌ 缺失 |
| `m_endTimeM` | int32_t | - | - | ❌ 缺失 |
| `m_endTimeExch` | int32_t | - | - | ❌ 缺失 |
| `m_endTime` | int64_t | - | - | ❌ 缺失 |
| `m_endTimeEpoch` | uint64_t | - | - | ❌ 缺失 |
| `m_endTimeAgg` | int64_t | - | - | ❌ 缺失 |
| `m_endTimeAggEpoch` | uint64_t | - | - | ❌ 缺失 |

### 11. 价格/Delta 相关变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_ltp` | double | - | - | ❌ 缺失 |
| `m_currAvgPrice` | double | - | - | ❌ 缺失 |
| `m_currPrice` | double | - | - | ❌ 缺失 |
| `m_targetPrice` | double | - | - | ❌ 缺失 |
| `m_currAvgDelta` | double | - | - | ❌ 缺失 |
| `m_currDelta` | double | - | - | ❌ 缺失 |
| `m_currAvgLoss` | double | - | - | ❌ 缺失 |
| `m_currLoss` | double | - | - | ❌ 缺失 |
| `m_indvalue` | double | - | - | ❌ 缺失 |
| `m_theoBid` | double | - | - | ❌ 缺失 |
| `m_theoAsk` | double | - | - | ❌ 缺失 |
| `m_lastTheoBid` | double | - | - | ❌ 缺失 |
| `m_lastTheoAsk` | double | - | - | ❌ 缺失 |
| `m_lastBid` | double | - | - | ❌ 缺失 |
| `m_lastAsk` | double | - | - | ❌ 缺失 |
| `m_lastTradePx` | double | - | - | ❌ 缺失 |
| `tmpAvgTargetPrice` | double | - | - | ❌ 缺失 |
| `tmpAvgDelta` | double | - | - | ❌ 缺失 |
| `tmpAvgLoss` | double | - | - | ❌ 缺失 |
| `m_targetBidPNL` | double* | - | - | ❌ 缺失 |
| `m_targetAskPNL` | double* | - | - | ❌ 缺失 |
| `bidOrderQty` | double | - | - | ❌ 缺失 |
| `bidOrderPx` | double | - | - | ❌ 缺失 |
| `askOrderQty` | double | - | - | ❌ 缺失 |
| `askOrderPx` | double | - | - | ❌ 缺失 |

### 12. 对冲相关变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_deltaBias` | double | - | - | ❌ 缺失 |
| `m_vegaBias` | double | - | - | ❌ 缺失 |
| `m_deltaAdj` | double | - | - | ❌ 缺失 |
| `m_vegaAdj` | double | - | - | ❌ 缺失 |
| `m_posAdj` | double | - | - | ❌ 缺失 |
| `m_positionBias` | double | - | - | ❌ 缺失 |
| `m_excessPosition` | double | - | - | ❌ 缺失 |
| `totalBiasAdj` | double | - | - | ❌ 缺失 |
| `hedgeBid` | double | - | - | ❌ 缺失 |
| `hedgeAsk` | double | - | - | ❌ 缺失 |
| `hedgeMid` | double | - | - | ❌ 缺失 |
| `hedgeScore` | double | - | - | ❌ 缺失 |
| `iocBias` | double | - | - | ❌ 缺失 |
| `iocPrice` | double | - | - | ❌ 缺失 |
| `iocScore` | double | - | - | ❌ 缺失 |
| `m_underlyingPredictedPrice` | double | - | - | ❌ 缺失 |

### 13. 其他引用变量

| C++ 变量 | C++ 类型 | Go 变量 | Go 位置 | 状态 |
|---------|---------|--------|---------|------|
| `m_instru` | Instrument* | `Instru` | ExtraStrategy | ✅ 已有 |
| `m_instru_sec` | Instrument* | - | - | ❌ 缺失 |
| `m_instru_third` | Instrument* | - | - | ❌ 缺失 |
| `m_client` | CommonClient* | - | - | N/A (不需要) |
| `m_simConfig` | SimConfig* | - | - | N/A (不需要) |
| `m_configParams` | ConfigParams* | - | - | N/A (不需要) |
| `m_strategyID` | int32_t | `StrategyID` | ExtraStrategy | ✅ 已有 |
| `m_instruStatMap` | InstruDistribMap* | - | - | ❌ 缺失 |
| `m_volParams` | VolParams* | - | - | ❌ 缺失 |
| `news_handler` | NewsHandler* | - | - | ❌ 缺失 |
| `m_tvar` | tvar\<double\>* | - | - | ❌ 缺失 |
| `m_tcache` | tcache\<double\>* | - | - | ❌ 缺失 |

---

## 方法对比

### 1. 核心回调方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `ORSCallBack(ResponseMsg*)` | `OnOrderUpdate(update)` | Strategy interface | ⚠️ 部分实现 |
| `MDCallBack(MarketUpdateNew*)` | `OnMarketData(md)` | Strategy interface | ✅ 已有 |
| `AuctionCallBack(MarketUpdateNew*)` | `OnAuctionData(md)` | Strategy interface | ✅ 已有 |
| `OnTradeUpdate()` | - | - | ❌ 缺失 |

### 2. 订单发送方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `SendOrder()` | - | - | ❌ 缺失 (纯虚函数) |
| `SendBidOrder(...)` | `SendBidOrder2(...)` | ExtraStrategy | ⚠️ 需要集成 |
| `SendAskOrder(...)` | `SendAskOrder2(...)` | ExtraStrategy | ⚠️ 需要集成 |
| `SendNewOrder(...)` | `SendNewOrder(...)` | ExtraStrategy | ⚠️ 部分实现 |
| `SendModifyOrder(...)` | `SendModifyOrder(...)` | ExtraStrategy | ⚠️ 部分实现 |
| `SendCancelOrder(orderID)` | `SendCancelOrder(orderID)` | ExtraStrategy | ⚠️ 未集成 ORS |
| `SendCancelOrder(price, side)` | `SendCancelOrderByPrice(...)` | ExtraStrategy | ⚠️ 未集成 ORS |
| `SendSweepOrder(...)` | `SendSweepOrder(...)` | ExtraStrategy | ⚠️ 部分实现 |

### 3. 订单回调处理方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `ProcessTrade(...)` | `ProcessTrade(...)` | ExtraStrategy | ✅ 已有 |
| `ProcessCancelConfirm(...)` | `ProcessCancelConfirm(...)` | ExtraStrategy | ✅ 已有 |
| `ProcessModifyConfirm(...)` | `ProcessModifyConfirm(...)` | ExtraStrategy | ✅ 已有 |
| `ProcessModifyReject(...)` | `ProcessModifyReject(...)` | ExtraStrategy | ✅ 已有 |
| `ProcessNewReject(...)` | `ProcessNewReject(...)` | ExtraStrategy | ✅ 已有 |
| `ProcessCancelReject(...)` | `ProcessCancelReject(...)` | ExtraStrategy | ⚠️ **未调用** |
| `ProcessSelfTrade(...)` | `ProcessSelfTrade(...)` | ExtraStrategy | ✅ 已有 |

### 4. 平仓/风控方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `HandleSquareoff()` | `HandleSquareoff()` | ExtraStrategy | ✅ 已有 |
| `HandleSquareON()` | `HandleSquareON()` | ExtraStrategy | ✅ 已有 |
| `HandleTimeLimitSquareoff()` | `HandleTimeLimitSquareoff()` | ExtraStrategy | ✅ 已有 |
| `CheckSquareoff(...)` | `CheckSquareoff(...)` | ExtraStrategy | ✅ 已有 |
| `SetCheckCancelQuantity()` | - | - | ❌ 缺失 |
| `SendAlert(...)` | - | - | ❌ 缺失 |

### 5. 阈值/价格方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `SetThresholds()` | `SetThresholds()` | ExtraStrategy | ✅ 已有 |
| `SetLinearThresholds()` | `SetLinearThresholds()` | ExtraStrategy | ✅ 已有 |
| `SetTargetValue(...)` | - | - | ❌ 缺失 |
| `GetBidPrice(...)` | `GetBidPrice(...)` | ExtraStrategy | ✅ 已有 |
| `GetAskPrice(...)` | `GetAskPrice(...)` | ExtraStrategy | ✅ 已有 |

### 6. 计算方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `CalculatePNL()` | `CalculatePNL(...)` | ExtraStrategy | ✅ 已有 |
| `CalculatePNL(buy, sell)` | - | - | ❌ 缺失 (重载版本) |

### 7. 订单映射方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `AddtoCache(...)` | - | - | ❌ 缺失 |
| `RemoveOrder(...)` | `RemoveFromOrderMap(...)` | ExtraStrategy | ✅ 已有 |
| `RemoveOpenDelta(...)` | - | - | ❌ 缺失 |
| `eraseFromOrderMap(...)` | `RemoveFromOrderMap(...)` | ExtraStrategy | ✅ 已有 |
| `SetQuantAhead(...)` | - | - | ❌ 缺失 |

### 8. 对冲方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `CancelPassiveHedgeOrders(...)` | - | - | ❌ 缺失 |
| `CancelIOCHedgeOrders()` | - | - | ❌ 缺失 |

### 9. 工具方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `Reset()` | `Reset()` | BaseStrategy | ✅ 已有 |
| `SendInLots()` | - | - | ❌ 缺失 |
| `RoundWorse(...)` | - | - | ❌ 缺失 |
| `GetOptionType(...)` | - | - | ❌ 缺失 |
| `GetInstrumentStats()` | - | - | ❌ 缺失 |

### 10. 监控/日志方法

| C++ 方法 | Go 方法 | 位置 | 状态 |
|---------|--------|------|------|
| `DumpOurBook()` | - | - | ❌ 缺失 |
| `DumpMktBook()` | - | - | ❌ 缺失 |
| `DumpStratBook()` | - | - | ❌ 缺失 |
| `DumpIndicators()` | - | - | ❌ 缺失 |
| `SendMonitorStratDetail(...)` | - | - | ❌ 缺失 |
| `SendMonitorStratPNL(...)` | - | - | ❌ 缺失 |
| `SendMonitorStratPos(...)` | - | - | ❌ 缺失 |
| `SendMonitorStratStatus(...)` | - | - | ❌ 缺失 |
| `SendMonitorStratCancelSts(...)` | - | - | ❌ 缺失 |

---

## 核心问题总结

### P0 - 严重问题 (阻塞性)

| 问题 | 说明 | 影响 |
|-----|------|------|
| **撤单请求未发送** | `SendCancelOrder` 只标记状态，未调用 `ORSClient.CancelOrder()` | 撤单功能无效 |
| **撤单拒绝未处理** | `ProcessCancelReject` 已定义但未被调用 | 撤单拒绝无法处理 |
| **订单发送未集成** | ExtraStrategy 的订单方法未与 ORSClient 集成 | 下单功能不完整 |

### P1 - 高优先级 (影响功能)

| 问题 | 说明 | 影响 |
|-----|------|------|
| 缺失 `m_rmsQty` | RMS 数量限制 | 风控缺失 |
| 缺失 `m_maxOrderCount` | 最大订单数限制 | 风控缺失 |
| 缺失 `m_maxPosSize` | 最大持仓限制 | 风控缺失 |
| 缺失时间控制变量 | `m_endTime*` 系列 | 收盘平仓逻辑缺失 |
| 缺失阈值控制变量 | `m_tholdBidSize` 等 | 分方向下单量控制缺失 |

### P2 - 中优先级 (影响完整性)

| 问题 | 说明 | 影响 |
|-----|------|------|
| 缺失交易费用变量 | `m_buyExchContractTx` 等 | 合约级费用计算缺失 |
| 缺失价格跟踪变量 | `m_currPrice`, `m_targetPrice` 等 | 价格分析缺失 |
| 缺失对冲相关变量 | Delta/Vega 偏差等 | 期权对冲功能缺失 |
| 缺失 Dump 方法 | 调试用订单簿输出 | 调试能力缺失 |

### P3 - 低优先级 (可选)

| 问题 | 说明 | 影响 |
|-----|------|------|
| 缺失监控方法 | `SendMonitorStrat*` 系列 | 监控系统集成缺失 |
| 缺失新闻处理 | `NewsHandler` | 新闻事件响应缺失 |
| 缺失期权相关 | `VolParams`, `OptionType` | 期权策略支持缺失 |
| 缺失统计队列 | `StatTradeQtyQ` 等 | 交易统计分析缺失 |

---

## 修复计划

### Phase 1: 订单流程修复 (P0)

**目标**: 修复撤单功能，确保订单生命周期完整

#### 1.1 撤单集成

**任务**: 将 `SendCancelOrder` 与 `ORSClient.CancelOrder` 集成

**涉及文件**:
- `golang/pkg/strategy/extra_strategy.go`
- `golang/pkg/strategy/engine.go`
- `golang/pkg/trader/trader.go`

**实现方案**:
```go
// engine.go 中添加撤单处理
func (se *StrategyEngine) processCancelRequests() {
    // 遍历策略，找出需要撤销的订单
    // 调用 ORSClient.CancelOrder()
    // 处理 CancelResponse
}
```

#### 1.2 撤单拒绝回调

**任务**: 在收到撤单拒绝时调用 `ProcessCancelReject`

**涉及文件**:
- `golang/pkg/strategy/engine.go`
- `golang/pkg/trader/trader.go`

**实现方案**:
```go
// 撤单响应处理
func (t *Trader) handleCancelResponse(resp *orspb.CancelResponse) {
    if resp.ErrorCode != orspb.ErrorCode_SUCCESS {
        // 撤单被拒绝
        strategy.ProcessCancelReject(orderID)
    }
}
```

#### 1.3 订单发送集成

**任务**: 确保 ExtraStrategy 的下单方法能实际发送到 ORS

**涉及文件**:
- `golang/pkg/strategy/extra_strategy.go`
- `golang/pkg/strategy/pairwise_arb_strategy.go`

### Phase 2: 风控变量补充 (P1)

**目标**: 补充风控相关缺失变量

#### 2.1 添加缺失变量到 ExtraStrategy

```go
// ExtraStrategy 补充字段
type ExtraStrategy struct {
    // ... 现有字段 ...

    // === 风控限制 (C++: ExecutionStrategy.h:109-110) ===
    RmsQty        int32  // m_rmsQty - RMS 数量限制
    MaxOrderCount uint64 // m_maxOrderCount - 最大订单数
    MaxPosSize    uint64 // m_maxPosSize - 最大持仓

    // === 时间控制 (C++: ExecutionStrategy.h:116-122) ===
    EndTimeH        int32  // m_endTimeH - 结束时
    EndTimeM        int32  // m_endTimeM - 结束分
    EndTime         int64  // m_endTime - 结束时间
    EndTimeEpoch    uint64 // m_endTimeEpoch - 结束时间戳
    EndTimeAgg      int64  // m_endTimeAgg - 主动平仓时间
    EndTimeAggEpoch uint64 // m_endTimeAggEpoch - 主动平仓时间戳

    // === 阈值控制 (C++: ExecutionStrategy.h:191-199) ===
    TholdMaxPos     int32 // m_tholdMaxPos - 最大持仓阈值
    TholdBeginPos   int32 // m_tholdBeginPos - 开始持仓阈值
    TholdInc        int32 // m_tholdInc - 增量阈值
    TholdSize       int32 // m_tholdSize - 单笔数量阈值
    TholdBidSize    int32 // m_tholdBidSize - 买单数量阈值
    TholdBidMaxPos  int32 // m_tholdBidMaxPos - 买单最大持仓
    TholdAskSize    int32 // m_tholdAskSize - 卖单数量阈值
    TholdAskMaxPos  int32 // m_tholdAskMaxPos - 卖单最大持仓
}
```

### Phase 3: 时间戳补充 (P2)

**目标**: 补充缺失的时间戳变量

```go
// ExtraStrategy 时间戳补充
type ExtraStrategy struct {
    // ... 现有字段 ...

    // === 额外时间戳 ===
    LastPosTS        uint64 // m_lastPosTS
    LastStsTS        uint64 // m_lastStsTS
    LastFlatTS       uint64 // m_lastFlatTS
    LastPxTS         uint64 // m_lastPxTS
    LastDeltaTS      uint64 // m_lastDeltaTS
    LastLossTS       uint64 // m_lastLossTS
    LastQtyTS        uint64 // m_lastQtyTS
    ExchTS           uint64 // m_exchTS
    LocalTS          uint64 // m_localTS
    LastSweepTradeTime uint64 // m_lastSweepTradeTime
}
```

### Phase 4: 价格/状态变量补充 (P2)

**目标**: 补充价格跟踪和状态变量

```go
// ExtraStrategy 价格和状态补充
type ExtraStrategy struct {
    // ... 现有字段 ...

    // === 价格跟踪 ===
    Ltp             float64 // m_ltp - 最新成交价
    CurrAvgPrice    float64 // m_currAvgPrice
    CurrPrice       float64 // m_currPrice
    TargetPrice     float64 // m_targetPrice
    TheoBid         float64 // m_theoBid
    TheoAsk         float64 // m_theoAsk
    LastTheoBid     float64 // m_lastTheoBid
    LastTheoAsk     float64 // m_lastTheoAsk
    LastBid         float64 // m_lastBid
    LastAsk         float64 // m_lastAsk
    LastTradePx     float64 // m_lastTradePx

    // === 状态标志补充 ===
    LastTradeSide      bool // m_lastTradeSide
    LastTrade          bool // m_lastTrade
    PendingBidCancel   bool // m_pendingBidCancel
    PendingAskCancel   bool // m_pendingAskCancel
    CheckCancelQuantity bool // m_checkCancelQuantity
    QuoteChanged       bool // quoteChanged
    IsBidOrderCrossing bool // isBidOrderCrossing
    IsAskOrderCrossing bool // isAskOrderCrossing
}
```

### Phase 5: 方法补充 (P2-P3)

**目标**: 补充缺失的工具和监控方法

#### 5.1 工具方法
- `RoundWorse(side, price, tick)` - 价格取整
- `SendInLots()` - 分批发送
- `SetCheckCancelQuantity()` - 设置撤单数量检查
- `AddtoCache(...)` - 订单缓存管理

#### 5.2 调试方法
- `DumpOurBook()` - 输出策略订单簿
- `DumpMktBook()` - 输出市场订单簿
- `DumpStratBook()` - 输出策略状态
- `DumpIndicators()` - 输出指标状态

---

## 优先级排序

| 优先级 | 任务 | 预估工作量 |
|-------|------|-----------|
| **P0** | 撤单集成 + 撤单拒绝回调 | 1天 |
| **P0** | 订单发送集成验证 | 0.5天 |
| **P1** | 风控变量补充 | 0.5天 |
| **P1** | 时间控制变量 + 收盘平仓逻辑 | 1天 |
| **P2** | 时间戳变量补充 | 0.5天 |
| **P2** | 价格/状态变量补充 | 0.5天 |
| **P2** | 调试方法补充 | 0.5天 |
| **P3** | 监控方法补充 | 1天 |
| **P3** | 对冲/期权变量 | 视需求 |

**总计**: 约 5-6 天（不含 P3 可选项）

---

## 参考资料

- C++ ExecutionStrategy: `/Users/user/PWorks/RD/tbsrc/Strategies/include/ExecutionStrategy.h`
- Go BaseStrategy: `/Users/user/PWorks/RD/quantlink-trade-system/golang/pkg/strategy/strategy.go`
- Go ExtraStrategy: `/Users/user/PWorks/RD/quantlink-trade-system/golang/pkg/strategy/extra_strategy.go`

---

**最后更新**: 2026-02-10 11:30
