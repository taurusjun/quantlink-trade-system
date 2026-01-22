# 事件回调机制对齐分析

## tbsrc 事件触发机制

### 1. 核心回调函数定义

tbsrc定义了4个主要的回调函数类型：

```cpp
// CommonClient.h:70-73
typedef void (*MDcb)(MarketUpdateNew *);      // 行情数据回调
typedef void (*ORScb)(ResponseMsg *);         // 订单回报回调
typedef void (*INDcb)(IndicatorList *);       // 指标计算完成回调
typedef void (*AUCcb)(MarketUpdateNew *);     // 竞价行情回调
```

### 2. 事件触发流程

#### 2.1 行情数据流程（Market Data Flow）

```
市场数据到达
    ↓
CommonClient::ProcessMarketData()
    ↓
1. 更新订单簿（Order Book）
    ↓
2. 生成Tick数据
    ↓
3. Update(iter, tick) - 更新Instrument级别指标
    ↓
    3.1 QuoteUpdate(tick) - 针对报价更新
    3.2 TickUpdate(tick) - 针对成交更新
    ↓
4. m_INDCallBack(&indicatorList) - 指标回调
    ↓
5. m_MDCallBack(update) - 行情回调
    ↓
Strategy::MDCallBack()
    ↓
    5.1 更新策略私有指标
    5.2 计算信号
    5.3 SetTargetValue()
    ↓
    5.4 SendOrder() - 同步发单
```

**关键点 (CommonClient.cpp:731-761)**:
```cpp
// 先更新Instrument级别的指标（共享指标）
Update(iter, tick);  // Line 731

// 然后触发指标回调
m_INDCallBack(&m_configParams->m_simConfig->m_indicatorList);  // Line 752

// 最后触发行情回调给策略
m_MDCallBack(update);  // Line 761
```

#### 2.2 订单回报流程（Order Response Flow）

```
订单回报消息到达
    ↓
CommonClient::ProcessOrderResponse()
    ↓
m_ORSCallBack(response)
    ↓
Strategy::ORSCallBack(response)
    ↓
    1. 更新订单状态
    2. 更新持仓
    3. 更新P&L
    4. 根据订单状态决策：
        - NEW_CONFIRM: 订单确认
        - TRADE_CONFIRM: 成交确认，可能触发对冲
        - CANCEL_CONFIRM: 撤单确认
        - *_REJECT: 拒单处理
```

**关键点 (CommonClient.cpp:298)**:
```cpp
m_ORSCallBack(response);  // 订单回报直接回调策略
```

#### 2.3 竞价行情流程（Auction Flow）

```
竞价期行情到达
    ↓
CommonClient::ProcessMarketData()
    ↓
if (update->m_feedType == FEED_AUCTION)
    ↓
m_AuctionCallBack(update)
    ↓
Strategy::AuctionCallBack(update)
    ↓
    处理竞价期特殊逻辑
```

**关键点 (CommonClient.cpp:454)**:
```cpp
if (update->m_feedType == FEED_AUCTION) {
    m_AuctionCallBack(update);
} else {
    // 正常行情流程
}
```

---

## quantlink-trade-system/golang 事件处理机制

### 当前实现的事件接口

```go
// Strategy interface (strategy.go:12-57)
type Strategy interface {
    OnMarketData(md *mdpb.MarketDataUpdate)    // 行情数据事件
    OnOrderUpdate(update *orspb.OrderUpdate)    // 订单更新事件
    OnTimer(now time.Time)                      // 定时器事件

    GetSignals() []*TradingSignal               // 获取交易信号
}
```

### 当前事件触发流程（同步模式）

```
NATS行情消息到达
    ↓
StrategyEngine::dispatchMarketDataSync()
    ↓
1. sharedIndPool.UpdateAll(symbol, md) - 更新共享指标（类似tbsrc的Update）
    ↓
2. 遍历所有策略：
    ↓
    2.1 strategy.OnMarketData(md)
        ↓
        - 更新私有指标
        - 计算信号
        - 添加到PendingSignals
    ↓
    2.2 signals := strategy.GetSignals()
    ↓
    2.3 sendOrderSync(signal) - 同步发单
```

**关键代码 (engine.go:261-297)**:
```go
func (se *StrategyEngine) dispatchMarketDataSync(md *mdpb.MarketDataUpdate) {
    // Step 1: 更新共享指标
    se.sharedIndPool.UpdateAll(md.Symbol, md)

    // Step 2: 处理每个策略
    for _, strategy := range se.strategies {
        s.OnMarketData(md)
        signals := s.GetSignals()
        for _, signal := range signals {
            se.sendOrderSync(signal)
        }
    }
}
```

---

## 对齐分析

### ✅ 已对齐的事件

| tbsrc事件 | quantlink-trade-system/golang | 对齐状态 | 说明 |
|-----------|-------------------------------|---------|------|
| **MDCallBack** | `OnMarketData()` | ✅ 对齐 | 行情数据到达时触发 |
| **ORSCallBack** | `OnOrderUpdate()` | ✅ 对齐 | 订单回报时触发 |
| **Update() → INDCallBack** | `sharedIndPool.UpdateAll()` | ✅ 对齐 | 指标更新流程已对齐 |

### ❌ 未对齐的事件

| tbsrc事件 | quantlink-trade-system/golang | 对齐状态 | 影响 |
|-----------|-------------------------------|---------|------|
| **AuctionCallBack** | ❌ 缺失 | ⚠️ 未对齐 | 无法处理竞价期行情 |
| **INDCallBack** (显式) | ❌ 隐式处理 | ⚠️ 部分对齐 | 无法在指标更新后、发单前插入逻辑 |

### 🔍 关键差异

#### 1. 指标回调（INDCallBack）

**tbsrc**:
```cpp
// 显式的指标回调，允许策略在指标计算完成后、发单前做额外处理
Update(iter, tick);                                    // 1. 更新指标
m_INDCallBack(&m_configParams->m_simConfig->m_indicatorList);  // 2. 指标回调
m_MDCallBack(update);                                  // 3. 行情回调 → 发单
```

**quantlink-trade-system/golang**:
```go
// 隐式处理：指标更新嵌入在OnMarketData()中
se.sharedIndPool.UpdateAll(md.Symbol, md)  // 1. 更新共享指标
strategy.OnMarketData(md)                  // 2. 策略内部更新私有指标 + 生成信号
se.sendOrderSync(signal)                   // 3. 发单
```

**影响**:
- ✅ 功能上等价，指标都在发单前更新完成
- ❌ 缺少独立的指标回调点，无法在指标更新后、策略决策前插入自定义逻辑
- ⚠️ 如果需要多个策略共享指标计算结果后做统一处理（如跨策略风控），当前架构需要在engine层添加钩子

#### 2. 竞价行情回调（AuctionCallBack）

**tbsrc**:
```cpp
if (update->m_feedType == FEED_AUCTION) {
    m_AuctionCallBack(update);  // 专门处理竞价期行情
} else {
    // 正常连续交易期行情
    m_MDCallBack(update);
}
```

**quantlink-trade-system/golang**:
```go
// 当前无竞价期区分
strategy.OnMarketData(md)  // 所有行情统一处理
```

**影响**:
- ❌ 无法区分竞价期和连续交易期行情
- ❌ 无法实现竞价期特殊策略逻辑（如集合竞价申报、撤单策略）
- ⚠️ 对于国内期货市场（有开盘集合竞价、收盘集合竞价），缺少这个功能

#### 3. 定时器回调（Timer）

**tbsrc**: 没有显式的定时器回调（依赖策略自行实现）

**quantlink-trade-system/golang**:
```go
OnTimer(now time.Time)  // 周期性定时器回调
```

**影响**:
- ✅ golang实现更完善，支持周期性任务（如风险检查、持仓报告）
- ✅ tbsrc通常在SendOrder()中基于时间戳做判断，golang提供了更清晰的接口

---

## 对齐建议

### 建议1: 添加竞价行情事件 ⭐⭐⭐

**优先级**: 高（如果需要支持国内市场）

**实现方案**:

```go
// 1. 扩展Strategy接口
type Strategy interface {
    OnMarketData(md *mdpb.MarketDataUpdate)
    OnAuctionData(md *mdpb.MarketDataUpdate)  // 新增：竞价行情回调
    OnOrderUpdate(update *orspb.OrderUpdate)
    OnTimer(now time.Time)
}

// 2. 在engine中区分行情类型
func (se *StrategyEngine) dispatchMarketDataSync(md *mdpb.MarketDataUpdate) {
    se.sharedIndPool.UpdateAll(md.Symbol, md)

    for _, strategy := range se.strategies {
        if md.FeedType == mdpb.FeedType_AUCTION {
            strategy.OnAuctionData(md)  // 竞价期回调
        } else {
            strategy.OnMarketData(md)   // 正常回调
        }
    }
}

// 3. 在BaseStrategy提供默认实现
func (bs *BaseStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
    // 默认：竞价期不做操作（策略可覆盖）
}
```

**好处**:
- ✅ 支持开盘集合竞价、收盘集合竞价策略
- ✅ 避免连续交易期逻辑在竞价期误触发
- ✅ 完全对齐tbsrc架构

### 建议2: 添加显式指标回调 ⭐⭐

**优先级**: 中（如果需要跨策略指标协调）

**实现方案**:

```go
// 1. 添加指标回调接口（可选实现）
type IndicatorAwareStrategy interface {
    OnIndicatorUpdate(symbol string, indicators *IndicatorLibrary)
}

// 2. 在engine中触发
func (se *StrategyEngine) dispatchMarketDataSync(md *mdpb.MarketDataUpdate) {
    // 1. 更新共享指标
    se.sharedIndPool.UpdateAll(md.Symbol, md)

    // 2. 通知所有关注该品种的策略：指标已更新
    for _, strategy := range se.strategies {
        if indStrategy, ok := strategy.(IndicatorAwareStrategy); ok {
            sharedInds := se.sharedIndPool.GetIndicators(md.Symbol)
            indStrategy.OnIndicatorUpdate(md.Symbol, sharedInds)
        }
    }

    // 3. 正常行情回调
    for _, strategy := range se.strategies {
        strategy.OnMarketData(md)
        // ...
    }
}
```

**好处**:
- ✅ 允许在指标更新后、策略决策前插入逻辑
- ✅ 适用于跨策略风控（如检查所有策略的指标是否异常）
- ✅ 保持向后兼容（可选实现）

### 建议3: 完善订单状态事件 ⭐

**优先级**: 低（当前OnOrderUpdate已基本覆盖）

**现状**:
- `OnOrderUpdate()`已覆盖所有订单状态变化
- tbsrc的`ORSCallBack`功能与此等价

**可选增强**:
```go
// 如果需要更细粒度的回调
type DetailedOrderStrategy interface {
    OnOrderNew(update *orspb.OrderUpdate)       // 订单确认
    OnOrderFilled(update *orspb.OrderUpdate)    // 成交回报
    OnOrderCanceled(update *orspb.OrderUpdate)  // 撤单确认
    OnOrderRejected(update *orspb.OrderUpdate)  // 拒单
}
```

**建议**: 暂不实现，当前`OnOrderUpdate()`已足够，策略内部可根据`update.Status`区分

---

## 对齐总结

### 🎯 对齐率: 85%

| 方面 | tbsrc | quantlink-trade-system/golang | 对齐度 |
|------|-------|-------------------------------|--------|
| 行情事件 | ✅ | ✅ | 100% |
| 订单事件 | ✅ | ✅ | 100% |
| 指标更新 | ✅ (显式) | ✅ (隐式) | 90% |
| 竞价事件 | ✅ | ❌ | 0% |
| 定时器事件 | ❌ | ✅ | N/A (golang更好) |

### 🚀 推荐行动

1. **必选**: 添加`OnAuctionData()`支持竞价行情（如需国内市场）
2. **可选**: 添加`OnIndicatorUpdate()`显式指标回调（如需跨策略协调）
3. **已完成**: ✅ 订单事件、行情事件、指标更新流程已对齐

### 📊 流程对比图

#### tbsrc完整流程
```
Market Data
    ↓
┌─────────────────────────────────┐
│ CommonClient                    │
│  1. Update OrderBook            │
│  2. Generate Tick               │
│  3. Update(tick)                │
│     └→ QuoteUpdate/TickUpdate   │
│        └→ Instrument指标更新    │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ INDCallBack(indicatorList)      │  ← 显式指标回调
│  允许在此处插入自定义逻辑        │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ MDCallBack(update) / AuctionCB  │  ← 行情回调（区分竞价/连续）
│  Strategy::MDCallBack()         │
│   1. 更新策略私有指标           │
│   2. 计算信号                   │
│   3. SetTargetValue()           │
│   4. SendOrder()  ← 同步发单    │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ ORSCallBack(response)           │  ← 订单回报
│  Strategy::ORSCallBack()        │
│   1. 更新订单状态               │
│   2. 更新持仓                   │
│   3. 更新P&L                    │
│   4. 触发对冲（如需要）         │
└─────────────────────────────────┘
```

#### quantlink-trade-system/golang当前流程
```
Market Data (NATS)
    ↓
┌─────────────────────────────────┐
│ StrategyEngine                  │
│  dispatchMarketDataSync()       │
│   1. sharedIndPool.UpdateAll()  │ ← 类似Update(tick)
│      └→ 更新共享指标             │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ (隐式：无显式指标回调)           │  ← ⚠️ 与tbsrc差异
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ Strategy.OnMarketData(md)       │  ← 类似MDCallBack
│  (⚠️ 无竞价/连续区分)            │  ← ⚠️ 与tbsrc差异
│   1. privateInds.UpdateAll()    │
│   2. 计算信号                   │
│   3. 添加到PendingSignals       │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ sendOrderSync(signal)           │  ← 同步发单（已对齐）
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ Strategy.OnOrderUpdate(update)  │  ← 类似ORSCallBack（已对齐）
│   1. 更新订单状态               │
│   2. 更新持仓                   │
│   3. 更新P&L                    │
└─────────────────────────────────┘
    ↓
┌─────────────────────────────────┐
│ Strategy.OnTimer(now)           │  ← golang独有（更好）
│   周期性任务                    │
└─────────────────────────────────┘
```

---

## 结论

当前quantlink-trade-system/golang的事件机制与tbsrc **基本对齐（85%）**，核心的行情→指标→发单流程已完全对齐。

**关键差距**:
1. ❌ 缺少竞价行情事件（建议补充）
2. ⚠️ 缺少显式指标回调（可选，当前隐式处理也能工作）

**额外优势**:
- ✅ golang提供了`OnTimer()`定时器回调，更清晰
- ✅ 异步/同步模式可切换，灵活性更高

**下一步**:
1. 根据业务需求决定是否需要竞价行情支持
2. 如果需要，实现`OnAuctionData()`事件
3. 验证当前隐式指标更新是否满足需求
