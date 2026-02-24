# 策略状态控制机制分析

## tbsrc 策略状态控制变量深度分析

### 核心状态控制变量

通过分析 `ExecutionStrategy.h/cpp`，发现tbsrc使用了一套精细的状态控制系统。

#### 1. 主要状态变量（ExecutionStrategy.h: 93-100）

```cpp
bool m_onExit;      // Line 93: 策略退出标志
bool m_onCancel;    // Line 94: 撤单标志（撤销所有在途订单）
bool m_onFlat;      // Line 96: 平仓标志（停止新订单）
bool m_aggFlat;     // Line 97: 激进平仓标志（穿越买卖盘）
bool m_Active;      // Line 100: 策略激活标志
```

#### 2. 辅助状态标志（ExecutionStrategy.h: 90-92）

```cpp
bool m_onMaxPx;     // Line 90: 最大价格触发标志
bool m_onNewsFlat;  // Line 91: 新闻事件平仓标志
bool m_onStopLoss;  // Line 92: 止损触发标志
bool m_onTimeSqOff; // Line 95: 时间强平标志
bool callSquareOff; // Line 101: 调用平仓标志
```

---

## 状态变量详细分析

### 状态1: `m_Active` - 策略激活标志

**用途**: 控制策略是否处于激活状态，只有激活状态下才能发单

**初始化**:
```cpp
// ExecutionStrategy.cpp: 377-380
if (m_configParams->m_modeType == ModeType_Sim)
    m_Active = true;  // 回测/模拟模式默认激活
else
    m_Active = false; // 实盘模式需要手动激活
```

**使用场景**:

1. **检查是否可以发单** (ExecutionStrategy.cpp: 454)
```cpp
if (!m_onFlat && m_Active) {
    // 策略激活且未平仓，可以发单
    SendOrder();
}
```

2. **策略退出时禁用** (ExecutionStrategy.cpp: 2367)
```cpp
if (m_onExit && m_Active) {
    TBLOG << "Positions Closed. Strategy Exiting.." << endl;
    m_Active = false;  // 退出时设为false
}
```

**关键判断条件**:
- `m_Active == true`: 策略正常运行
- `m_Active == false`: 策略已停止，不发送新订单

---

### 状态2: `m_onFlat` - 平仓标志

**用途**: 停止发送新订单，准备平仓（但不立即撤单）

**触发条件**:

1. **止损触发** (ExecutionStrategy.cpp: 2279-2282)
```cpp
if ((m_unrealisedPNL < m_thold->UPNL_LOSS * -1 ||
     m_netPNL < m_thold->STOP_LOSS * -1) &&
    !m_onCancel && !m_onFlat) {

    m_onStopLoss = true;
    m_onFlat = true;  // 触发平仓
    // ...
}
```

2. **价格超出范围** (ExecutionStrategy.cpp: 2245-2248)
```cpp
if ((m_currAvgPrice < m_thold->MIN_PRICE ||
     m_currAvgPrice > m_thold->MAX_PRICE) &&
    !m_onFlat) {
    m_onFlat = true;  // 价格异常，平仓
}
```

3. **Delta超限** (ExecutionStrategy.cpp: 2211-2214)
```cpp
if (((abs(m_currAvgDelta) < minDelta ||
      abs(m_currAvgDelta) > maxDelta)) &&
    !m_onFlat) {
    m_onCancel = true;
    m_onFlat = true;   // Delta异常，平仓
}
```

4. **新闻事件** (ExecutionStrategy.cpp: 2304-2307)
```cpp
if (!m_onCancel && !m_onFlat && news_handler->getFlat()) {
    m_onNewsFlat = true;
    m_onCancel = true;
    m_onFlat = true;  // 新闻事件，立即平仓
}
```

**恢复条件**:

```cpp
// ExecutionStrategy.cpp: 2267-2270
// 价格回到正常范围后可以恢复
if ((m_currAvgPrice > m_thold->MIN_PRICE &&
     m_currAvgPrice < m_thold->MAX_PRICE) &&
    m_onFlat && !m_onExit) {
    m_onCancel = false;
    m_onFlat = false;  // 恢复正常交易
}

// 止损后15分钟可以恢复
if (m_exchTS - m_lastFlatTS > 900000000000 && // 15 mins
    !m_onExit && m_onStopLoss) {
    m_onFlat = false;
    m_onStopLoss = false;  // 恢复
}
```

**行为**:
- `m_onFlat == true`: 停止发送新订单，但不主动撤单
- `m_onFlat == false`: 可以正常发单

---

### 状态3: `m_onCancel` - 撤单标志

**用途**: 撤销所有在途订单（通常与 `m_onFlat` 一起使用）

**使用场景**:

```cpp
// ExecutionStrategy.cpp: 2391, 2400
// HandleSquareoff() 函数中
for (PriceMapIter iter = m_askMap.begin(); iter != m_askMap.end(); iter++) {
    if (m_onCancel || /* other conditions */) {
        SendCancelOrder(iter->second->m_orderID);  // 撤销订单
    }
}

// 撤单完成后重置标志
m_onCancel = false;  // Line 2408
```

**常见组合**:
```cpp
// 止损时同时触发撤单和平仓
m_onCancel = true;
m_onFlat = true;
```

**行为**:
- `m_onCancel == true`: 立即撤销所有挂单
- 撤单完成后自动重置为 `false`

---

### 状态4: `m_onExit` - 策略退出标志

**用途**: 策略完全退出，关闭所有持仓并停止运行

**触发条件**:

1. **时间到期** (ExecutionStrategy.cpp: 2161-2164)
```cpp
if ((Watch::GetUniqueInstance()->GetCurrentTime() >= m_endTimeEpoch ||
     m_netPNL < m_thold->MAX_LOSS * -1 ||
     m_orderCount >= m_maxOrderCount) &&
    !m_onExit) {

    m_onExit = true;
    m_onCancel = true;
    m_onFlat = true;  // 三个标志同时设置
}
```

2. **拒单过多** (ExecutionStrategy.cpp: 474-480)
```cpp
if (m_rejectCount > REJECT_LIMIT) {
    m_onCancel = true;
    m_onFlat = true;
    m_Active = false;  // 停止策略
}
```

3. **竞价期（特定交易所）** (ExecutionStrategy.cpp: 762-766)
```cpp
void AuctionCallBack(MarketUpdateNew *up) {
    if ((m_client->m_Mode == ModeType_Live) &&
        (!m_onExit || !m_onFlat)) {
        m_onExit = true;
        m_onCancel = true;
        m_onFlat = true;  // 竞价期停止交易
    }
}
```

**完全退出条件**:

```cpp
// ExecutionStrategy.cpp: 2361-2368
if (m_netpos == 0 &&      // 仓位已平
    m_onExit &&
    m_askMap.size() == 0 &&  // 无挂卖单
    m_bidMap.size() == 0) {  // 无挂买单

    if (m_onExit && m_Active) {
        m_Active = false;  // 完全停止策略
    }
}
```

**行为**:
- `m_onExit == true`: 策略进入退出流程
- 一旦设置，不可恢复（不像 `m_onFlat` 可以恢复）
- 必须等待所有持仓平仓、所有订单取消后，才真正停止策略

---

### 状态5: `m_aggFlat` - 激进平仓标志

**用途**: 激进模式平仓，穿越买卖盘以快速平仓

**使用场景**:

```cpp
// ExecutionStrategy.cpp: 2372-2373
// 激进平仓时使用不利价格
double sellprice = m_aggFlat ? m_instru->bidPx[0] - m_instru->m_tickSize  // 更低卖价
                              : m_instru->askPx[0];                         // 正常卖价

double buyprice = m_aggFlat ? m_instru->askPx[0] + m_instru->m_tickSize   // 更高买价
                             : m_instru->bidPx[0];                          // 正常买价

// 发送穿越订单
if (m_aggFlat)
    SendNewOrder(SELL, sellprice, qty, 0, QUOTE, CROSS);  // 穿越卖单
```

**行为**:
- `m_aggFlat == true`: 使用不利价格快速平仓（吃单）
- `m_aggFlat == false`: 使用挂单方式平仓（做市）

---

## 状态转换流程图

```
正常运行状态:
    m_Active = true
    m_onFlat = false
    m_onCancel = false
    m_onExit = false
    ↓
    [正常发单]
    ↓

风险触发 (止损/价格异常/Delta异常):
    m_onFlat = true         ← 停止新订单
    m_onCancel = true       ← 撤销在途订单
    m_onStopLoss = true     ← 记录原因
    ↓
    [平仓流程]
    ↓

条件恢复 (价格回归/时间冷却):
    m_onFlat = false        ← 恢复发单
    m_onCancel = false
    ↓
    [返回正常运行]

严重风险 (时间到期/亏损过大/拒单过多):
    m_onExit = true         ← 进入退出流程
    m_onCancel = true
    m_onFlat = true
    ↓
    [强制平仓]
    ↓
    持仓清零 && 无挂单:
    m_Active = false        ← 策略完全停止
    ↓
    [策略结束]
```

---

## quantlink-trade-system/golang 当前状态管理

### 现有状态 (strategy.go: 100-101)

```go
type BaseStrategy struct {
    // ... other fields
    IsRunningFlag      bool                            // 类似 m_Active
    // 缺失：m_onFlat, m_onCancel, m_onExit, m_aggFlat
}
```

**现有问题**:
1. ❌ 只有 `IsRunningFlag`，功能过于简单
2. ❌ 无法区分"暂停交易"vs"完全退出"
3. ❌ 无法实现"风险触发后恢复"逻辑
4. ❌ 无法支持激进平仓模式
5. ❌ 无法记录触发原因（止损/价格/新闻）

---

## 对齐建议

### 建议1: 添加完整状态控制机制 ⭐⭐⭐

**优先级**: 高（生产环境必需）

**实现方案**:

```go
// 策略运行状态枚举
type StrategyRunState int

const (
    StrategyRunStateActive      StrategyRunState = iota // 正常运行
    StrategyRunStatePaused                              // 暂停（风险触发）
    StrategyRunStateFlattening                          // 平仓中
    StrategyRunStateExiting                             // 退出中
    StrategyRunStateStopped                             // 已停止
)

// 平仓原因枚举
type FlattenReason int

const (
    FlattenReasonNone           FlattenReason = iota
    FlattenReasonStopLoss                      // 止损
    FlattenReasonPriceLimit                    // 价格超限
    FlattenReasonDeltaLimit                    // Delta超限
    FlattenReasonTimeLimit                     // 时间到期
    FlattenReasonRejectLimit                   // 拒单过多
    FlattenReasonNewsEvent                     // 新闻事件
    FlattenReasonManual                        // 手动平仓
)

// 策略控制状态
type StrategyControlState struct {
    RunState       StrategyRunState  // 运行状态
    FlattenMode    bool              // 平仓模式（类似 m_onFlat）
    CancelPending  bool              // 待撤单（类似 m_onCancel）
    ExitRequested  bool              // 退出请求（类似 m_onExit）
    AggressiveFlat bool              // 激进平仓（类似 m_aggFlat）

    FlattenReason  FlattenReason     // 平仓原因
    FlattenTime    time.Time         // 平仓触发时间
    CanRecoverAt   time.Time         // 可恢复时间（用于冷却）
}

// 添加到 BaseStrategy
type BaseStrategy struct {
    // ... existing fields
    ControlState   StrategyControlState  // 新增：状态控制
}
```

**关键方法**:

```go
// 触发平仓
func (bs *BaseStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
    bs.ControlState.FlattenMode = true
    bs.ControlState.CancelPending = true
    bs.ControlState.AggressiveFlat = aggressive
    bs.ControlState.FlattenReason = reason
    bs.ControlState.FlattenTime = time.Now()
    bs.ControlState.RunState = StrategyRunStateFlattening

    // 根据原因设置恢复时间
    switch reason {
    case FlattenReasonStopLoss:
        bs.ControlState.CanRecoverAt = time.Now().Add(15 * time.Minute)  // 止损后15分钟可恢复
    case FlattenReasonPriceLimit:
        bs.ControlState.CanRecoverAt = time.Now().Add(1 * time.Minute)   // 价格恢复后1分钟可尝试
    default:
        bs.ControlState.CanRecoverAt = time.Time{}  // 无法自动恢复
    }
}

// 尝试恢复
func (bs *BaseStrategy) TryRecover() bool {
    if bs.ControlState.ExitRequested {
        return false  // 已请求退出，不可恢复
    }

    if bs.ControlState.FlattenMode &&
       time.Now().After(bs.ControlState.CanRecoverAt) &&
       bs.Position.IsFlat() {  // 仓位已平

        bs.ControlState.FlattenMode = false
        bs.ControlState.CancelPending = false
        bs.ControlState.AggressiveFlat = false
        bs.ControlState.RunState = StrategyRunStateActive
        return true
    }
    return false
}

// 触发退出
func (bs *BaseStrategy) TriggerExit(reason string) {
    bs.ControlState.ExitRequested = true
    bs.ControlState.FlattenMode = true
    bs.ControlState.CancelPending = true
    bs.ControlState.RunState = StrategyRunStateExiting
    log.Printf("[%s] Strategy exiting: %s", bs.ID, reason)
}

// 检查是否可以发单
func (bs *BaseStrategy) CanSendOrder() bool {
    return bs.ControlState.RunState == StrategyRunStateActive &&
           !bs.ControlState.FlattenMode &&
           !bs.ControlState.ExitRequested &&
           bs.IsRunningFlag
}

// 检查是否需要撤单
func (bs *BaseStrategy) ShouldCancelOrders() bool {
    return bs.ControlState.CancelPending
}
```

---

### 建议2: 集成到Engine的风险检查 ⭐⭐

**优先级**: 中（增强风控）

**实现方案**:

```go
// engine.go
func (se *StrategyEngine) OnTimer(now time.Time) {
    se.mu.RLock()
    defer se.mu.RUnlock()

    for _, strategy := range se.strategies {
        // 检查风险限制
        if err := strategy.CheckRiskLimits(); err != nil {
            log.Printf("[%s] Risk limit exceeded: %v", strategy.GetID(), err)

            // 根据错误类型触发不同的平仓模式
            if accessor, ok := strategy.(BaseStrategyAccessor); ok {
                baseStrat := accessor.GetBaseStrategy()

                // 判断风险类型
                if strings.Contains(err.Error(), "stop_loss") {
                    baseStrat.TriggerFlatten(FlattenReasonStopLoss, false)
                } else if strings.Contains(err.Error(), "reject_limit") {
                    baseStrat.TriggerExit("Too many rejections")
                }
            }
        }

        // 尝试恢复
        if accessor, ok := strategy.(BaseStrategyAccessor); ok {
            baseStrat := accessor.GetBaseStrategy()
            if baseStrat.ControlState.FlattenMode {
                if baseStrat.TryRecover() {
                    log.Printf("[%s] Strategy recovered from flatten mode", strategy.GetID())
                }
            }
        }

        // 调用策略的定时器
        strategy.OnTimer(now)
    }
}
```

---

### 建议3: 平仓逻辑实现 ⭐⭐

**优先级**: 中（完善平仓机制）

**实现方案**:

```go
// strategy.go
func (bs *BaseStrategy) HandleFlatten(currentPrice float64) {
    if !bs.ControlState.FlattenMode {
        return
    }

    // Step 1: 撤销所有挂单
    if bs.ControlState.CancelPending {
        // TODO: 调用engine的撤单接口
        bs.ControlState.CancelPending = false
    }

    // Step 2: 如果有持仓，生成平仓信号
    if !bs.Position.IsFlat() {
        var signal *TradingSignal

        if bs.Position.IsLong() {
            // 平多仓：卖出
            price := currentPrice
            if bs.ControlState.AggressiveFlat {
                // 激进模式：穿越买盘
                price = currentPrice - bs.Config.TickSize
            }

            signal = &TradingSignal{
                StrategyID: bs.ID,
                Symbol:     bs.Config.Symbol,
                Side:       orspb.OrderSide_SELL,
                Qty:        bs.Position.LongQty,
                Price:      price,
                Type:       orspb.OrderType_LIMIT,
                Timestamp:  time.Now(),
            }
        } else {
            // 平空仓：买入
            price := currentPrice
            if bs.ControlState.AggressiveFlat {
                // 激进模式：穿越卖盘
                price = currentPrice + bs.Config.TickSize
            }

            signal = &TradingSignal{
                StrategyID: bs.ID,
                Symbol:     bs.Config.Symbol,
                Side:       orspb.OrderSide_BUY,
                Qty:        bs.Position.ShortQty,
                Price:      price,
                Type:       orspb.OrderType_LIMIT,
                Timestamp:  time.Now(),
            }
        }

        bs.AddSignal(signal)
    }

    // Step 3: 检查是否可以完全退出
    if bs.ControlState.ExitRequested &&
       bs.Position.IsFlat() &&
       len(bs.PendingSignals) == 0 {
        bs.ControlState.RunState = StrategyRunStateStopped
        bs.IsRunningFlag = false
        log.Printf("[%s] Strategy fully stopped", bs.ID)
    }
}
```

---

## 对比总结

| 状态控制 | tbsrc | quantlink (当前) | 建议实现 |
|---------|-------|-----------------|---------|
| 基本运行控制 | m_Active | IsRunningFlag | ✅ 已有 |
| 平仓模式 | m_onFlat | ❌ | ⭐⭐⭐ 需要 |
| 撤单控制 | m_onCancel | ❌ | ⭐⭐⭐ 需要 |
| 退出控制 | m_onExit | ❌ | ⭐⭐⭐ 需要 |
| 激进平仓 | m_aggFlat | ❌ | ⭐⭐ 需要 |
| 状态原因记录 | 多个bool标志 | ❌ | ⭐⭐ 需要 |
| 自动恢复机制 | ✅ | ❌ | ⭐⭐ 需要 |
| 定时器检查 | CheckSquareoff | ❌ | ⭐⭐ 需要 |

---

## 使用场景示例

### 场景1: 止损触发

**tbsrc**:
```cpp
if (m_netPNL < m_thold->STOP_LOSS * -1) {
    m_onStopLoss = true;
    m_onCancel = true;
    m_onFlat = true;
    HandleSquareoff();  // 立即平仓
}
```

**golang (建议)**:
```go
if strategy.GetPNL().NetPnL < strategy.Config.RiskLimits["stop_loss"] * -1 {
    strategy.TriggerFlatten(FlattenReasonStopLoss, false)
    strategy.HandleFlatten(currentPrice)
}
```

### 场景2: 时间到期退出

**tbsrc**:
```cpp
if (currentTime >= m_endTimeEpoch && !m_onExit) {
    m_onExit = true;
    m_onCancel = true;
    m_onFlat = true;
}
```

**golang (建议)**:
```go
if time.Now().After(strategy.Config.EndTime) && !strategy.ControlState.ExitRequested {
    strategy.TriggerExit("Trading time ended")
}
```

### 场景3: 风险恢复

**tbsrc**:
```cpp
// 价格回到正常范围后自动恢复
if ((m_currAvgPrice > m_thold->MIN_PRICE &&
     m_currAvgPrice < m_thold->MAX_PRICE) &&
    m_onFlat && !m_onExit) {
    m_onFlat = false;
    m_onCancel = false;
}
```

**golang (建议)**:
```go
// 定时器中自动尝试恢复
if strategy.ControlState.FlattenMode &&
   time.Now().After(strategy.ControlState.CanRecoverAt) {
    if strategy.TryRecover() {
        log.Printf("Strategy %s recovered", strategy.ID)
    }
}
```

---

## 实施优先级

### 必选（⭐⭐⭐）
1. **FlattenMode**: 平仓模式控制
2. **ExitRequested**: 退出请求控制
3. **CanSendOrder()**: 发单前状态检查

### 推荐（⭐⭐）
4. **AggressiveFlat**: 激进平仓模式
5. **FlattenReason**: 原因记录和监控
6. **TryRecover()**: 自动恢复机制

### 可选（⭐）
7. **详细状态枚举**: 更细粒度的状态管理
8. **状态历史记录**: 用于回测分析

---

## 总结

tbsrc使用了一套成熟的多状态控制系统，包括：
- **5个主要状态标志**: m_Active, m_onFlat, m_onCancel, m_onExit, m_aggFlat
- **多个辅助标志**: m_onStopLoss, m_onNewsFlat, etc.
- **自动恢复机制**: 风险解除后可恢复交易
- **状态转换逻辑**: 清晰的状态转换流程

quantlink-trade-system/golang当前只有简单的 `IsRunningFlag`，**严重缺失**：
- ❌ 平仓模式控制
- ❌ 退出流程管理
- ❌ 风险自动恢复
- ❌ 激进平仓支持

**建议尽快实现核心状态控制机制（⭐⭐⭐优先级），以支持生产环境的风险管理需求。**
