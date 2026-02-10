# ExtraStrategy/PairwiseArb 代码一致性修复计划

**文档日期**: 2026-02-10
**作者**: Claude
**版本**: v2.0
**相关文档**: @docs/系统分析/ExtraStrategy_PairwiseArb_Go与CPP代码对比_2026-02-10-00_30.md

---

## 概述

根据 Go 与 C++ 代码对比分析，本计划列出**所有**需要修复的不一致项（❌ 未实现 和 ⚠️ 部分实现），按优先级分阶段实施。

**统计**:
- ExtraStrategy 变量: 17 个未实现 + 4 个部分实现
- ExtraStrategy 方法: 14 个未实现 + 3 个部分实现
- PairwiseArbStrategy 变量: 10 个未实现 + 1 个部分实现
- PairwiseArbStrategy 方法: 7 个未实现 + 4 个部分实现
- **总计**: 48 个未实现 + 12 个部分实现 = **60 项修复**

---

## 修复优先级说明

| 优先级 | 说明 | 标准 |
|--------|------|------|
| P0 | 紧急 | 影响核心交易逻辑，可能导致持仓/订单错误 |
| P1 | 高 | 影响追单、风控等重要功能 |
| P2 | 中 | 影响统计、监控等辅助功能 |
| P3 | 低 | 优化项，不影响主要功能 |

---

## Phase 1: 核心功能修复 (P0) - 7 项

### 1.1 添加 ProcessCancelReject 方法 ❌

**问题**: 撤单被拒绝时没有处理逻辑，可能导致订单状态不一致

**C++ 原代码**: `ExecutionStrategy::ProcessCancelReject()`

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// ProcessCancelReject 处理撤单拒绝
// C++: ExecutionStrategy::ProcessCancelReject()
func (es *ExtraStrategy) ProcessCancelReject(orderID uint32) {
    es.mu.Lock()
    defer es.mu.Unlock()

    if orderStats, exists := es.OrdMap[orderID]; exists {
        orderStats.Status = OrderStatusCancelReject
        orderStats.Cancel = false
        es.RejectCount++
        es.LastCancelRejectTime = uint64(time.Now().UnixNano())
        es.LastCancelRejectOrderID = orderID
    }
}
```

### 1.2 添加 tholdSecond 阈值配置 ❌

**问题**: 第二条腿没有独立的阈值配置

**C++ 原代码**: `m_thold_second` in `PairwiseArbStrategy.h:66`

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 结构体中添加
tholdSecond *ThresholdSet // 第二条腿阈值配置

// 在 Initialize() 中初始化
pas.tholdSecond = NewThresholdSet()
pas.secondStrat.Thold = pas.tholdSecond
```

### 1.3 修复 NetPosAgg 更新逻辑 ⚠️

**问题**: ProcessTrade 中没有区分被动单和主动单更新 NetPosPass/NetPosAgg

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ProcessTrade 中根据订单类型更新不同字段
if orderStats.OrdType == OrderHitTypeCross || orderStats.OrdType == OrderHitTypeMatch {
    // 主动单更新 NetPosAgg
    if side == TransactionTypeBuy {
        es.NetPosAgg += filledQty
    } else {
        es.NetPosAgg -= filledQty
    }
} else {
    // 被动单更新 NetPosPass
    if side == TransactionTypeBuy {
        es.NetPosPass += filledQty
    } else {
        es.NetPosPass -= filledQty
    }
}
```

### 1.4 完善 ThresholdSet 阈值字段 ❌⚠️

**问题**: 缺少 `BidSize`, `AskSize`，`BidMaxSize`/`AskMaxSize` 部分实现

**修改文件**: `golang/pkg/strategy/threshold_set.go`

```go
// 在 ThresholdSet 中添加
BidSize    int32 // m_tholdBidSize - 买单单笔限额
AskSize    int32 // m_tholdAskSize - 卖单单笔限额
// BidMaxSize, AskMaxSize 已存在，确保逻辑正确使用
```

### 1.5 添加撤单拒绝时间戳字段 ❌

**问题**: 缺少撤单拒绝相关的时间戳和订单ID跟踪

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
LastCancelRejectTime    uint64 // m_lastCancelRejectTime
LastCancelRejectOrderID uint32 // m_lastCancelRejectOrderID
```

### 1.6 添加 avgSpreadRatio 计算字段 ⚠️

**问题**: `avgSpreadRatio` 通过 tValue 调整，但没有显式字段

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 中添加辅助方法
func (pas *PairwiseArbStrategy) getAvgSpreadRatio() float64 {
    // C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
    return pas.spreadAnalyzer.GetStats().Mean + pas.tValue
}
```

### 1.7 添加风控字段 ❌

**问题**: 缺少 `m_maxloss_limit` 和 `is_valid_mkdata`

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 结构体中添加
maxLossLimit  float64 // m_maxloss_limit - 最大亏损限制
isValidMkdata bool    // is_valid_mkdata - 行情数据是否有效
```

---

## Phase 2: 追单和风控修复 (P1) - 12 项

### 2.1 添加 HandleSquareoff 平仓处理 ❌

**修改文件**:
- `golang/pkg/strategy/extra_strategy.go`
- `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// ExtraStrategy.HandleSquareoff
func (es *ExtraStrategy) HandleSquareoff() {
    es.mu.Lock()
    defer es.mu.Unlock()
    es.OnFlat = true
    for orderID, order := range es.OrdMap {
        if order.Active {
            es.SendCancelOrder(orderID)
        }
    }
}

// PairwiseArbStrategy.HandleSquareoff
func (pas *PairwiseArbStrategy) HandleSquareoff() {
    pas.firstStrat.HandleSquareoff()
    pas.secondStrat.HandleSquareoff()
    pas.generateExitSignals(nil)
}
```

### 2.2 添加 HandleSquareON 方法 ❌

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// HandleSquareON 开启平仓模式
func (pas *PairwiseArbStrategy) HandleSquareON() {
    pas.firstStrat.OnFlat = true
    pas.secondStrat.OnFlat = true
    log.Printf("[PairwiseArb:%s] Square mode ON", pas.ID)
}
```

### 2.3 添加 ProcessModifyConfirm/Reject 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// ProcessModifyConfirm 处理改单确认
func (es *ExtraStrategy) ProcessModifyConfirm(orderID uint32, newPrice float64, newQty int32) {
    es.mu.Lock()
    defer es.mu.Unlock()
    if orderStats, exists := es.OrdMap[orderID]; exists {
        if orderStats.Side == TransactionTypeBuy {
            delete(es.BidMap, orderStats.Price)
            es.BidMap[newPrice] = orderStats
        } else {
            delete(es.AskMap, orderStats.Price)
            es.AskMap[newPrice] = orderStats
        }
        orderStats.OldPrice = orderStats.Price
        orderStats.Price = newPrice
        orderStats.Qty = newQty
        orderStats.OpenQty = newQty
        orderStats.Status = OrderStatusModifyConfirm
        orderStats.ModifyWait = false
    }
}

// ProcessModifyReject 处理改单拒绝
func (es *ExtraStrategy) ProcessModifyReject(orderID uint32) {
    es.mu.Lock()
    defer es.mu.Unlock()
    if orderStats, exists := es.OrdMap[orderID]; exists {
        orderStats.Status = OrderStatusModifyReject
        orderStats.ModifyWait = false
        orderStats.NewPrice = orderStats.Price
        es.RejectCount++
    }
}
```

### 2.4 添加 SendModifyOrder 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// SendModifyOrder 发送改单请求
func (es *ExtraStrategy) SendModifyOrder(orderID uint32, newPrice float64, newQty int32) (*TradingSignal, bool) {
    es.mu.Lock()
    defer es.mu.Unlock()
    orderStats, exists := es.OrdMap[orderID]
    if !exists || !orderStats.Active || orderStats.ModifyWait || orderStats.Cancel {
        return nil, false
    }
    orderStats.OldPrice = orderStats.Price
    orderStats.OldQty = orderStats.Qty
    orderStats.NewPrice = newPrice
    orderStats.NewQty = newQty
    orderStats.Status = OrderStatusModifyOrder
    orderStats.ModifyWait = true
    return &TradingSignal{
        OrderID:   fmt.Sprintf("%d", orderID),
        Side:      orderStats.Side,
        Price:     newPrice,
        Quantity:  int64(newQty),
        OrderType: OrderTypeModify,
    }, true
}
```

### 2.5 添加 ProcessSelfTrade 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// ProcessSelfTrade 处理自成交
// C++: ExecutionStrategy::ProcessSelfTrade()
func (es *ExtraStrategy) ProcessSelfTrade(orderID uint32) {
    es.mu.Lock()
    defer es.mu.Unlock()
    // 自成交处理：通常需要撤单并记录
    if orderStats, exists := es.OrdMap[orderID]; exists {
        orderStats.Active = false
        log.Printf("[ExtraStrategy:%d] Self-trade detected, orderID=%d", es.StrategyID, orderID)
    }
}
```

### 2.6 添加 SetLinearThresholds 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// SetLinearThresholds 设置线性阈值
// C++: ExecutionStrategy::SetLinearThresholds()
func (es *ExtraStrategy) SetLinearThresholds() {
    // 线性阈值计算逻辑
}
```

### 2.7 添加 HandleTimeLimitSquareoff 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// HandleTimeLimitSquareoff 时间限制平仓
// C++: ExecutionStrategy::HandleTimeLimitSquareoff()
func (es *ExtraStrategy) HandleTimeLimitSquareoff() {
    es.mu.Lock()
    defer es.mu.Unlock()
    es.OnTimeSqOff = true
    es.HandleSquareoff()
}
```

### 2.8 添加 CheckSquareoff 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// CheckSquareoff 检查是否需要平仓
// C++: ExecutionStrategy::CheckSquareoff()
func (es *ExtraStrategy) CheckSquareoff() bool {
    if es.OnStopLoss {
        return true
    }
    if es.Thold != nil && es.NetPNL < -es.Thold.MaxLoss {
        return true
    }
    return false
}
```

### 2.9 添加 SendSweepOrder 方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// SendSweepOrder 发送扫单
// C++: ExecutionStrategy::SendSweepOrder()
func (es *ExtraStrategy) SendSweepOrder(price float64, side TransactionType) (*TradingSignal, bool) {
    // 扫单逻辑：以市价或激进价格快速成交
}
```

### 2.10 添加价格计算方法 ⚠️

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// GetBidPrice_first 获取第一条腿买单挂单价格
// C++: PairwiseArbStrategy::GetBidPrice_first()
func (pas *PairwiseArbStrategy) GetBidPrice_first(level int) (price float64, ordType OrderHitType) {
    if level >= len(pas.bidPrices1) || pas.bidPrices1[level] <= 0 {
        return 0, OrderHitTypeStandard
    }
    price = pas.bidPrices1[level]
    ordType = OrderHitTypeStandard

    // 隐性订单簿检测
    if pas.enablePriceOptimize && level > 0 {
        prevPrice := pas.bidPrices1[level-1]
        tickSize := pas.tickSize1
        if price < prevPrice-tickSize {
            bidInv := price - pas.bid2 + tickSize
            spreadMean := pas.getAvgSpreadRatio()
            if bidInv <= spreadMean-pas.tholdFirst.BeginPlace {
                if pas.firstStrat.HasOrderAtPrice(price, TransactionTypeBuy) {
                    price = price + tickSize
                }
            }
        }
    }
    return price, ordType
}

// GetAskPrice_first 获取第一条腿卖单挂单价格
func (pas *PairwiseArbStrategy) GetAskPrice_first(level int) (price float64, ordType OrderHitType) {
    // ... 类似实现
}

// GetBidPrice_second 获取第二条腿买单挂单价格
func (pas *PairwiseArbStrategy) GetBidPrice_second(level int) (price float64, ordType OrderHitType) {
    // ... 类似实现
}

// GetAskPrice_second 获取第二条腿卖单挂单价格
func (pas *PairwiseArbStrategy) GetAskPrice_second(level int) (price float64, ordType OrderHitType) {
    // ... 类似实现
}
```

### 2.11 添加 GetBidPrice/GetAskPrice 基础方法 ❌

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// GetBidPrice 获取买单挂单价格
// C++: ExecutionStrategy::GetBidPrice()
func (es *ExtraStrategy) GetBidPrice(level int32) (float64, OrderHitType, int32) {
    // 基础价格获取逻辑
}

// GetAskPrice 获取卖单挂单价格
// C++: ExecutionStrategy::GetAskPrice()
func (es *ExtraStrategy) GetAskPrice(level int32) (float64, OrderHitType, int32) {
    // 基础价格获取逻辑
}
```

### 2.12 添加 CalculatePNL 方法到 ExtraStrategy ⚠️

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// CalculatePNL 计算盈亏
// C++: ExecutionStrategy::CalculatePNL()
func (es *ExtraStrategy) CalculatePNL(bidPrice, askPrice float64) {
    es.mu.Lock()
    defer es.mu.Unlock()

    if es.NetPos > 0 {
        // 多头：用卖价计算
        es.UnrealisedPNL = float64(es.NetPos) * (bidPrice - es.BuyAvgPrice)
    } else if es.NetPos < 0 {
        // 空头：用买价计算
        es.UnrealisedPNL = float64(-es.NetPos) * (es.SellAvgPrice - askPrice)
    }
    es.NetPNL = es.RealisedPNL + es.UnrealisedPNL
}
```

---

## Phase 3: 统计和字段修复 (P2) - 23 项

### 3.1 订单统计字段 ❌ (5项)

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
CancelConfirmCount int32 // m_cancelconfirmCount - 撤单确认次数
PriceCount         int32 // m_priceCount - 价格变动次数
DeltaCount         int32 // m_deltaCount - Delta 变动次数
LossCount          int32 // m_lossCount - 亏损次数
QtyCount           int32 // m_qtyCount - 数量变动次数
```

### 3.2 成交量统计字段 ❌ (4项)

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
BuyExchTx  float64 // m_buyExchTx - 买入交易所手续费
SellExchTx float64 // m_sellExchTx - 卖出交易所手续费
BuyValue   float64 // m_buyValue - 买入价值（当前持仓）
SellValue  float64 // m_sellValue - 卖出价值（当前持仓）
```

### 3.3 状态标志字段 ❌ (3项)

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
OnMaxPx     bool // m_onMaxPx - 达到最大价格
OnNewsFlat  bool // m_onNewsFlat - 新闻触发平仓
OnTimeSqOff bool // m_onTimeSqOff - 时间触发平仓
```

### 3.4 时间戳字段 ❌ (3项)

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
LastHBTS     uint64 // m_lastHBTS - 最后心跳时间
LastOrdTS    uint64 // m_lastOrdTS - 最后订单时间戳
LastDetailTS uint64 // m_lastDetailTS - 最后详情时间戳
```

### 3.5 订单映射缓存 ❌ (3项)

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// 在 ExtraStrategy 结构体中添加
SweepOrdMap    map[uint32]*OrderStats  // m_sweepordMap - 扫单映射
BidMapCache    map[float64]*OrderStats // m_bidMapCache - 买单缓存
AskMapCache    map[float64]*OrderStats // m_askMapCache - 卖单缓存
```

### 3.6 PairwiseArb 价格字段 ❌ (4项)

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 结构体中添加
currSpreadRatioPrev float64 // currSpreadRatio_prev - 前一价差
expectedRatio       float64 // expectedRatio - 期望比率
iu                  float64 // iu
count               float64 // count
```

### 3.7 追单字段 ❌ (1项)

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 结构体中添加
secondOrdIDStart float64 // second_ordIDstart - 第二腿订单ID起始
```

---

## Phase 4: 辅助功能修复 (P3) - 10 项

### 4.1 矩阵操作方法 ❌ (5项)

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// LoadMatrix 加载矩阵
func (pas *PairwiseArbStrategy) LoadMatrix(filepath string) map[string]map[string]float64 {
    // 从文件加载矩阵数据
}

// LoadMatrix2 加载矩阵2
func (pas *PairwiseArbStrategy) LoadMatrix2(filepath string) map[int32]map[string]string {
    // 从文件加载矩阵数据
}

// SaveMatrix 保存矩阵
func (pas *PairwiseArbStrategy) SaveMatrix(filepath string, matrix map[string]map[string]float64) {
    // 保存矩阵数据到文件
}

// SaveMatrix2 保存矩阵2
func (pas *PairwiseArbStrategy) SaveMatrix2(filepath string) {
    // 保存矩阵数据到文件
}

// SendTCacheLeg1Pos 发送Leg1持仓到缓存
func (pas *PairwiseArbStrategy) SendTCacheLeg1Pos() {
    // 发送持仓到 tCache
}
```

### 4.2 矩阵数据字段 ❌ (1项)

**修改文件**: `golang/pkg/strategy/pairwise_arb_strategy.go`

```go
// 在 PairwiseArbStrategy 结构体中添加
mxDailyInit map[string]map[string]float64 // mx_daily_init - 每日初始化矩阵
```

### 4.3 SendNewOrder 独立方法 ⚠️

**问题**: 逻辑在 SendBidOrder2/SendAskOrder2 中，但没有独立的 SendNewOrder

**修改文件**: `golang/pkg/strategy/extra_strategy.go`

```go
// SendNewOrder 发送新订单（底层方法）
// C++: ExtraStrategy::SendNewOrder()
func (es *ExtraStrategy) SendNewOrder(side TransactionType, price float64, qty int32, level int32, ordType OrderHitType) (*TradingSignal, *OrderStats) {
    // 创建 OrderStats
    orderStats := NewOrderStats()
    orderStats.Side = side
    orderStats.Price = price
    orderStats.Qty = qty
    orderStats.OpenQty = qty
    orderStats.OrdType = ordType
    orderStats.Status = OrderStatusNewOrder

    // 确定信号类别
    category := SignalCategoryPassive
    if ordType == OrderHitTypeCross || ordType == OrderHitTypeMatch {
        category = SignalCategoryAggressive
    }

    var orderSide OrderSide
    if side == TransactionTypeBuy {
        orderSide = OrderSideBuy
    } else {
        orderSide = OrderSideSell
    }

    signal := &TradingSignal{
        Side:       orderSide,
        Price:      price,
        Quantity:   int64(qty),
        Category:   category,
        QuoteLevel: int(level),
    }

    es.OrderCount++
    return signal, orderStats
}
```

### 4.4 MDCallBack 独立方法 ⚠️

**问题**: 行情回调逻辑在 PairwiseArbStrategy.OnMarketData() 中

**说明**: 这是架构差异，Go 使用组合而非继承，此项可保持现状，但需要确保 ExtraStrategy 可以独立处理行情

```go
// MDCallBack 行情回调（可选实现）
// 当前逻辑已在 PairwiseArbStrategy.OnMarketData() 中处理
func (es *ExtraStrategy) MDCallBack(bidPrice, askPrice float64) {
    es.CalculatePNL(bidPrice, askPrice)
    es.SetThresholds()
}
```

### 4.5 旧版订单方法（低优先级）❌

**说明**: `SendBidOrder()` 和 `SendAskOrder()` 是旧版本方法，已被 SendBidOrder2/SendAskOrder2 替代

**决定**: 暂不实现，保持使用新版本方法

---

## 实施时间表

| 阶段 | 内容 | 任务数 | 优先级 |
|------|------|--------|--------|
| Phase 1 | 核心功能修复 | 7 项 | P0 |
| Phase 2 | 追单和风控修复 | 12 项 | P1 |
| Phase 3 | 统计和字段修复 | 23 项 | P2 |
| Phase 4 | 辅助功能修复 | 10 项 | P3 |
| **总计** | | **52 项** | |

---

## 修复清单汇总

### ExtraStrategy 字段 (21项)

| 字段 | 状态 | Phase |
|------|------|-------|
| `CancelConfirmCount` | ❌ | 3 |
| `PriceCount` | ❌ | 3 |
| `DeltaCount` | ❌ | 3 |
| `LossCount` | ❌ | 3 |
| `QtyCount` | ❌ | 3 |
| `BuyExchTx` | ❌ | 3 |
| `SellExchTx` | ❌ | 3 |
| `BuyValue` | ❌ | 3 |
| `SellValue` | ❌ | 3 |
| `SweepOrdMap` | ❌ | 3 |
| `BidMapCache` | ❌ | 3 |
| `AskMapCache` | ❌ | 3 |
| `BidSize` | ❌ | 1 |
| `AskSize` | ❌ | 1 |
| `BidMaxSize` | ⚠️ | 1 |
| `AskMaxSize` | ⚠️ | 1 |
| `OnMaxPx` | ❌ | 3 |
| `OnNewsFlat` | ❌ | 3 |
| `OnTimeSqOff` | ❌ | 3 |
| `LastHBTS` | ❌ | 3 |
| `LastOrdTS` | ❌ | 3 |
| `LastDetailTS` | ❌ | 3 |
| `LastCancelRejectTime` | ❌ | 1 |
| `LastCancelRejectOrderID` | ❌ | 1 |

### ExtraStrategy 方法 (14项)

| 方法 | 状态 | Phase |
|------|------|-------|
| `SendModifyOrder()` | ❌ | 2 |
| `SendSweepOrder()` | ❌ | 2 |
| `SendNewOrder()` | ⚠️ | 4 |
| `ProcessModifyConfirm()` | ❌ | 2 |
| `ProcessModifyReject()` | ❌ | 2 |
| `ProcessCancelReject()` | ❌ | 1 |
| `ProcessSelfTrade()` | ❌ | 2 |
| `SetLinearThresholds()` | ❌ | 2 |
| `HandleSquareoff()` | ❌ | 2 |
| `HandleTimeLimitSquareoff()` | ❌ | 2 |
| `CheckSquareoff()` | ❌ | 2 |
| `CalculatePNL()` | ⚠️ | 2 |
| `GetBidPrice()` | ❌ | 2 |
| `GetAskPrice()` | ❌ | 2 |
| `MDCallBack()` | ⚠️ | 4 |

### PairwiseArbStrategy 字段 (11项)

| 字段 | 状态 | Phase |
|------|------|-------|
| `tholdSecond` | ❌ | 1 |
| `avgSpreadRatio` | ⚠️ | 1 |
| `currSpreadRatioPrev` | ❌ | 3 |
| `expectedRatio` | ❌ | 3 |
| `iu` | ❌ | 3 |
| `count` | ❌ | 3 |
| `secondOrdIDStart` | ❌ | 3 |
| `maxLossLimit` | ❌ | 1 |
| `isValidMkdata` | ❌ | 1 |
| `mxDailyInit` | ❌ | 4 |

### PairwiseArbStrategy 方法 (11项)

| 方法 | 状态 | Phase |
|------|------|-------|
| `HandleSquareON()` | ❌ | 2 |
| `HandleSquareoff()` | ❌ | 2 |
| `GetBidPrice_first()` | ⚠️ | 2 |
| `GetAskPrice_first()` | ⚠️ | 2 |
| `GetBidPrice_second()` | ⚠️ | 2 |
| `GetAskPrice_second()` | ⚠️ | 2 |
| `LoadMatrix()` | ❌ | 4 |
| `LoadMatrix2()` | ❌ | 4 |
| `SaveMatrix()` | ❌ | 4 |
| `SaveMatrix2()` | ❌ | 4 |
| `SendTCacheLeg1Pos()` | ❌ | 4 |

---

## 验证方案

### 单元测试

```bash
# 运行 ExtraStrategy 测试
go test -v ./pkg/strategy/... -run "TestExtraStrategy"

# 运行 PairwiseArbStrategy 测试
go test -v ./pkg/strategy/... -run "TestPairwiseArb"
```

### 端到端测试

```bash
# 模拟测试
./scripts/test/e2e/test_simulator_e2e.sh

# CTP 实盘测试
./scripts/test/e2e/test_ctp_live_e2e.sh --run
```

### 验证检查点

**Phase 1 检查点**:
- [ ] ProcessCancelReject 正确处理撤单拒绝
- [ ] tholdSecond 独立配置生效
- [ ] NetPosPass/NetPosAgg 分别更新
- [ ] BidSize/AskSize 字段生效
- [ ] 风控字段 maxLossLimit 生效

**Phase 2 检查点**:
- [ ] HandleSquareoff 正确触发平仓
- [ ] HandleSquareON 开启平仓模式
- [ ] SendModifyOrder 改单功能正常
- [ ] ProcessModifyConfirm/Reject 正确处理
- [ ] GetBidPrice_first/second 正确优化价格
- [ ] GetAskPrice_first/second 正确优化价格
- [ ] CalculatePNL 正确计算盈亏

**Phase 3 检查点**:
- [ ] 新增统计字段正确累计
- [ ] 时间戳字段正确更新
- [ ] 缓存映射正常工作

**Phase 4 检查点**:
- [ ] 矩阵加载/保存正常
- [ ] SendTCacheLeg1Pos 正常发送

---

## 参考文档

- 对比分析: @docs/系统分析/ExtraStrategy_PairwiseArb_Go与CPP代码对比_2026-02-10-00_30.md
- C++ ExecutionStrategy: `tbsrc/Strategies/include/ExecutionStrategy.h`
- C++ ExtraStrategy: `tbsrc/Strategies/include/ExtraStrategy.h`
- C++ PairwiseArbStrategy: `tbsrc/Strategies/PairwiseArbStrategy.cpp`
- 重构计划: @.claude/plans/luminous-meandering-clarke.md

---

**最后更新**: 2026-02-10 00:50
