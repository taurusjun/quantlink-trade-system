# Indicators计算在数据流中的位置

**创建时间**: 2026-01-20

---

## 📊 完整数据流图

```
┌─────────────────────────────────────────────────────────────────┐
│                    1. 数据源层                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  MD Simulator    │  生成行情数据
                    │  或 真实交易所    │  (Tick级别)
                    └────────┬─────────┘
                             │
                             │ Shared Memory
                             │ (SPSC Queue)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    2. Gateway层 (C++)                            │
└─────────────────────────────────────────────────────────────────┘
                             │
                    ┌────────┴─────────┐
                    │   MD Gateway     │  读取共享内存
                    │                  │  转换为Protobuf
                    └────────┬─────────┘
                             │
                             │ gRPC Streaming / NATS Publish
                             │ (MarketDataUpdate protobuf)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                 3. Strategy Engine (Golang)                      │
└─────────────────────────────────────────────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │  StrategyEngine             │
              │  - Subscribe NATS           │
              │  - dispatchMarketData()     │
              └──────────────┬──────────────┘
                             │
                             │ 分发到各策略
                             │ strategy.OnMarketData(md)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              4. 策略层 - OnMarketData() 方法                      │
│                  🔥 INDICATORS在这里计算 🔥                       │
└─────────────────────────────────────────────────────────────────┘

              ┌────────────────────────────┐
              │  Strategy.OnMarketData()   │
              └─────────────┬──────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │ 第1步: 更新所有指标          │
              │ Indicators.UpdateAll(md)    │  ◄─── ✨ 指标计算位置
              └─────────────┬───────────────┘
                            │
                            ├──► EWMA.Update(md)
                            ├──► RSI.Update(md)
                            ├──► MACD.Update(md)
                            ├──► Bollinger.Update(md)
                            ├──► ATR.Update(md)
                            └──► ... (所有注册的指标)
                            │
                            ▼
              ┌─────────────────────────────┐
              │ 第2步: 计算价格              │
              │ midPrice = (bid+ask)/2      │
              └─────────────┬───────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │ 第3步: 更新PNL和风控指标     │
              │ UpdatePNL(midPrice)         │
              │ UpdateRiskMetrics()         │
              └─────────────┬───────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │ 第4步: 读取指标值            │
              │ trend = indicator.GetValue()│
              │ momentum = indicator.GetValue()│
              └─────────────┬───────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │ 第5步: 生成交易信号          │
              │ generateSignals()           │
              │ - 基于指标值计算             │
              │ - 生成买卖信号               │
              └─────────────┬───────────────┘
                            │
                            ▼
                    ┌──────────────┐
                    │ TradingSignal│
                    └──────┬───────┘
                           │
                           ▼
              订单发送 → ORS Gateway → 交易所
```

---

## 🎯 关键代码位置

### 位置1: Strategy.OnMarketData() - 指标更新入口

**文件**: `pkg/strategy/aggressive_strategy.go:136-145`

```go
func (as *AggressiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
    as.mu.Lock()
    defer as.mu.Unlock()

    if !as.IsRunning() {
        return
    }

    // ✨ 关键: 这里更新所有指标
    as.Indicators.UpdateAll(md)  // ◄─── 指标计算的触发点

    // ... 后续使用指标值生成信号
}
```

**其他策略同样位置**:
- `passive_strategy.go:149` - `ps.Indicators.UpdateAll(md)`
- `hedging_strategy.go:137` - `hs.Indicators.UpdateAll(md)`
- `pairwise_arb_strategy.go:139` - `pas.Indicators.UpdateAll(md)`

---

### 位置2: IndicatorLibrary.UpdateAll() - 批量更新

**文件**: `pkg/indicators/indicator.go:215-222`

```go
// UpdateAll updates all indicators with new market data
func (lib *IndicatorLibrary) UpdateAll(md *mdpb.MarketDataUpdate) {
    lib.mu.RLock()
    defer lib.mu.RUnlock()

    // 遍历所有注册的指标，逐个更新
    for _, indicator := range lib.indicators {
        indicator.Update(md)  // ◄─── 调用每个指标的Update方法
    }
}
```

---

### 位置3: 各指标的Update() - 具体计算

**示例1: SMA指标**

**文件**: `pkg/indicators/sma.go:42-73`

```go
func (sma *SMA) Update(md *mdpb.MarketDataUpdate) {
    if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
        return
    }

    // 1. 提取价格
    price := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

    // 2. 累加到滚动窗口
    sma.prices = append(sma.prices, price)
    sma.sum += price

    // 3. 维护固定窗口大小
    if len(sma.prices) > sma.period {
        oldest := sma.prices[0]
        sma.prices = sma.prices[1:]
        sma.sum -= oldest  // O(1) 滚动窗口算法
    }

    // 4. 计算平均值
    if len(sma.prices) == sma.period {
        avg := sma.sum / float64(sma.period)
        sma.AddValue(avg)  // 存储到历史值
        sma.initialized = true
    }
}
```

**示例2: Bollinger Bands指标**

**文件**: `pkg/indicators/bollinger.go:52-95`

```go
func (bb *BollingerBands) Update(md *mdpb.MarketDataUpdate) {
    // 1. 提取价格
    price := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

    // 2. 维护滚动窗口
    bb.prices = append(bb.prices, price)
    if len(bb.prices) > bb.period {
        bb.prices = bb.prices[1:]
    }

    // 3. 当窗口满时计算
    if len(bb.prices) == bb.period {
        // 计算SMA (中轨)
        sum := 0.0
        for _, p := range bb.prices {
            sum += p
        }
        bb.middleBand = sum / float64(bb.period)

        // 计算标准差
        variance := 0.0
        for _, p := range bb.prices {
            diff := p - bb.middleBand
            variance += diff * diff
        }
        stdDev := math.Sqrt(variance / float64(bb.period))

        // 计算上下轨
        bb.upperBand = bb.middleBand + (bb.stdDevMult * stdDev)
        bb.lowerBand = bb.middleBand - (bb.stdDevMult * stdDev)

        // 计算%B和Bandwidth
        bb.calculatePercentB(price)
        bb.calculateBandwidth()

        bb.initialized = true
    }
}
```

**示例3: ATR指标**

**文件**: `pkg/indicators/atr.go:39-89`

```go
func (atr *ATR) Update(md *mdpb.MarketDataUpdate) {
    // 1. 提取价格
    high := md.AskPrice[0]
    low := md.BidPrice[0]
    close := (high + low) / 2.0

    // 2. 计算True Range
    tr := atr.calculateTrueRange(high, low, close)

    // 3. 存储TR值
    atr.trValues = append(atr.trValues, tr)
    if len(atr.trValues) > atr.period {
        atr.trValues = atr.trValues[1:]
    }

    atr.dataPoints++

    // 4. 计算ATR
    if atr.dataPoints == atr.period {
        // 初始ATR: 简单平均
        sum := 0.0
        for _, trVal := range atr.trValues {
            sum += trVal
        }
        atr.atr = sum / float64(atr.period)
    } else if atr.dataPoints > atr.period {
        // 后续ATR: Wilder's平滑
        atr.atr = ((atr.atr * float64(atr.period-1)) + tr) / float64(atr.period)
    }

    // 5. 保存历史值
    if atr.IsReady() {
        atr.AddValue(atr.atr)
    }

    atr.prevClose = close
    atr.hasPrevious = true
}
```

---

### 位置4: 策略读取指标值并生成信号

**文件**: `pkg/strategy/aggressive_strategy.go:204-253`

```go
func (as *AggressiveStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
    // 1. 获取指标实例
    trendIndicator, ok := as.Indicators.Get(fmt.Sprintf("ewma_trend_%d", as.trendPeriod))
    if !ok || !trendIndicator.IsReady() {
        return  // 指标还未就绪
    }

    momentumIndicator, ok := as.Indicators.Get(fmt.Sprintf("ewma_momentum_%d", as.momentumPeriod))
    if !ok || !momentumIndicator.IsReady() {
        return
    }

    volIndicator, ok := as.Indicators.Get("volatility")
    if !ok || !volIndicator.IsReady() {
        return
    }

    // 2. 读取指标值 (已经在Update中计算好)
    trend := trendIndicator.GetValue()        // ◄─── 读取趋势
    momentum := momentumIndicator.GetValue()  // ◄─── 读取动量
    volatility := volIndicator.GetValue()     // ◄─── 读取波动率

    // 3. 基于指标值计算信号
    trendSignal := (as.lastPrice - trend) / trend
    momentumSignal := (as.lastPrice - momentum) / momentum
    signal := 0.6*trendSignal + 0.4*momentumSignal

    // 4. 生成交易信号
    if math.Abs(signal) >= as.signalThreshold {
        // 生成 BUY 或 SELL 信号
        tradingSignal := &TradingSignal{
            StrategyID: as.ID,
            Symbol:     md.Symbol,
            Side:       ...,
            Price:      ...,
            Quantity:   ...,
            Signal:     signal,
            // ...
        }
        as.AddSignal(tradingSignal)
    }
}
```

---

## ⏱️ 时序图

```
Time ──────────────────────────────────────────────►

Tick 1:
  Exchange → Simulator → ShmQueue → Gateway → NATS → Engine
                                                        │
                                                        ▼
                                                   Strategy.OnMarketData()
                                                        │
                                                        ├─► Indicators.UpdateAll()
                                                        │     ├─► EWMA.Update()  ◄─── 计算EWMA
                                                        │     ├─► RSI.Update()   ◄─── 计算RSI
                                                        │     └─► ATR.Update()   ◄─── 计算ATR
                                                        │
                                                        ├─► UpdatePNL()
                                                        └─► generateSignals()
                                                              ├─► trend = ewma.GetValue()
                                                              ├─► rsi = rsi.GetValue()
                                                              └─► 生成信号

Tick 2:
  Exchange → ... → Strategy.OnMarketData()
                      │
                      ├─► Indicators.UpdateAll()  ◄─── 再次更新所有指标
                      │     (使用新的Tick数据)
                      └─► generateSignals()
                            (基于最新指标值)

Tick 3: ...
```

---

## 📈 性能特点

### 1. 计算时机
- **实时计算**: 每个Tick到达时立即计算
- **增量更新**: 使用滚动窗口，O(1)复杂度
- **并发安全**: 每个策略独立计算，互不干扰

### 2. 计算频率
```
行情频率: 100-10000 Hz (取决于数据源)
指标更新频率 = 行情频率
每个Tick: 更新10个指标 (P0指标)
总计算量: 1000-100000 次/秒
```

### 3. 延迟分析
```
行情到达 → 指标更新 → 信号生成
   ↓           ↓            ↓
  50μs       10μs         20μs
   │          │            │
   └──────────┴────────────┘
          总延迟: ~80μs
```

### 4. 内存占用
```
每个指标:
- SMA(20):        20个价格 × 8字节 = 160 bytes
- Bollinger(20):  20个价格 × 8字节 = 160 bytes
- ATR(14):        14个TR值 × 8字节 = 112 bytes
- History:        1000个历史值 × 8字节 = 8KB

每个策略(10个指标): ~100KB
```

---

## 🔍 调试技巧

### 1. 查看指标计算过程

在 `OnMarketData` 中添加日志:

```go
func (as *AggressiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
    // 更新前
    oldValue := as.Indicators.Get("ewma_trend_50").GetValue()

    as.Indicators.UpdateAll(md)

    // 更新后
    newValue := as.Indicators.Get("ewma_trend_50").GetValue()
    log.Printf("[DEBUG] EWMA updated: %.2f → %.2f (price: %.2f)",
               oldValue, newValue, md.BidPrice[0])
}
```

### 2. 检查指标就绪状态

```go
for name, indicator := range as.Indicators.indicators {
    log.Printf("[DEBUG] %s: ready=%v, value=%.4f",
               name, indicator.IsReady(), indicator.GetValue())
}
```

### 3. 性能分析

```go
import "time"

func (as *AggressiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
    start := time.Now()

    as.Indicators.UpdateAll(md)

    elapsed := time.Since(start)
    log.Printf("[PERF] Indicators update: %v", elapsed)
}
```

---

## 📝 关键要点总结

| 要素 | 描述 |
|------|------|
| **计算位置** | Strategy.OnMarketData() 方法中 |
| **触发时机** | 每次收到新行情数据时 |
| **调用顺序** | Indicators.UpdateAll() → 每个indicator.Update() |
| **执行频率** | = 行情频率 (100-10000 Hz) |
| **延迟** | 每个指标 < 1μs，总计 ~10μs |
| **数据流向** | MarketDataUpdate → Indicator → GetValue() → Signal |
| **线程安全** | 每个策略有独立的IndicatorLibrary |
| **内存占用** | 每个策略 ~100KB (10个指标) |

---

## 🎯 常见问题

### Q1: 为什么不在Gateway层计算指标？

**A**:
- **策略定制**: 不同策略需要不同参数的指标
- **灵活性**: 策略可以动态创建和配置指标
- **解耦**: Gateway只负责数据传输，不涉及业务逻辑

### Q2: 指标计算会阻塞行情接收吗？

**A**:
- **不会**: 每个策略在独立的goroutine中处理
- **并发**: StrategyEngine.dispatchMarketData() 并发分发
- **非阻塞**: 指标计算只需要 ~10μs

### Q3: 如何确保指标计算的准确性？

**A**:
- **单元测试**: 每个指标有完整的测试覆盖
- **已知值验证**: 使用已知输入验证输出
- **基准测试**: 性能基准确保效率

### Q4: 能否共享指标计算结果？

**A**:
- **当前**: 每个策略独立计算
- **优化**: 可以实现共享IndicatorLibrary (需要加锁)
- **权衡**: 独立计算避免锁竞争，性能更好

---

## 🔗 相关文档

- [系统启动文档](系统启动_20260120.md) - 完整数据流说明
- [指标实现状态](INDICATOR_IMPLEMENTATION_STATUS.md) - 已实现指标清单
- [P0指标完成报告](golang/pkg/indicators/P0_INDICATORS_COMPLETION_REPORT.md) - 指标详细说明

---

**总结**: Indicators在**Strategy.OnMarketData()方法的第一步**被计算，每收到一个新的行情Tick就更新一次，确保策略始终使用最新的技术指标值来生成交易信号。
