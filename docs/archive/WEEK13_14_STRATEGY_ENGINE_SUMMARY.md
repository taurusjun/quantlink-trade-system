# Week 13-14: 策略引擎实现总结

## 完成时间
2026-01-20

## 概述
按照统一架构设计文档，完成了Week 13-14的策略引擎框架和核心策略实现。

## 已完成工作

### 1. 策略类型定义 (`pkg/strategy/types.go`)

#### 核心数据结构

**TradingSignal** - 交易信号:
```go
type TradingSignal struct {
    StrategyID  string      // 策略ID
    Symbol      string      // 合约代码
    Side        OrderSide   // 买卖方向
    Price       float64     // 价格
    Quantity    int64       // 数量
    Signal      float64     // 信号强度 [-1, 1]
    Confidence  float64     // 置信度 [0, 1]
    Timestamp   time.Time   // 生成时间
    Metadata    map[string]interface{} // 元数据
}
```

**Position** - 仓位管理:
```go
type Position struct {
    LongQty       int64   // 多头仓位
    ShortQty      int64   // 空头仓位
    NetQty        int64   // 净仓位
    AvgLongPrice  float64 // 平均买入价
    AvgShortPrice float64 // 平均卖出价
    RealizedPnL   float64 // 已实现盈亏
    UnrealizedPnL float64 // 未实现盈亏
}
```

**PNL** - 盈亏跟踪:
```go
type PNL struct {
    RealizedPnL   float64 // 已实现盈亏
    UnrealizedPnL float64 // 未实现盈亏
    TotalPnL      float64 // 总盈亏
    TradingFees   float64 // 交易费用
    NetPnL        float64 // 净盈亏
    MaxDrawdown   float64 // 最大回撤
}
```

**RiskMetrics** - 风险指标:
```go
type RiskMetrics struct {
    PositionSize    int64   // 当前仓位
    MaxPositionSize int64   // 最大允许仓位
    ExposureValue   float64 // 敞口价值
    MaxExposure     float64 // 最大允许敞口
    VaR             float64 // 在险价值
    MaxDrawdown     float64 // 最大回撤
}
```

### 2. 策略接口和基类 (`pkg/strategy/strategy.go`)

#### Strategy接口

定义了所有策略必须实现的方法：
```go
type Strategy interface {
    Initialize(config *StrategyConfig) error
    Start() error
    Stop() error
    IsRunning() bool

    // 事件回调
    OnMarketData(md *mdpb.MarketDataUpdate)
    OnOrderUpdate(update *orspb.OrderUpdate)
    OnTimer(now time.Time)

    // 状态查询
    GetSignals() []*TradingSignal
    GetPosition() *Position
    GetPNL() *PNL
    GetRiskMetrics() *RiskMetrics
    GetStatus() *StrategyStatus

    Reset()
}
```

#### BaseStrategy基类

提供通用功能：
- **仓位管理**: 自动更新多空仓位、平均价格
- **盈亏计算**: 实时计算已实现和未实现盈亏
- **风险检查**: 检查仓位限制、敞口限制、回撤限制
- **信号队列**: 管理待发送的交易信号
- **订单跟踪**: 跟踪所有订单状态
- **指标库集成**: 内置IndicatorLibrary

**关键方法**:
```go
func (bs *BaseStrategy) UpdatePosition(update *orspb.OrderUpdate)
func (bs *BaseStrategy) UpdatePNL(currentPrice float64)
func (bs *BaseStrategy) UpdateRiskMetrics(currentPrice float64)
func (bs *BaseStrategy) CheckRiskLimits() bool
func (bs *BaseStrategy) AddSignal(signal *TradingSignal)
```

### 3. PassiveStrategy实现 (`pkg/strategy/passive_strategy.go`)

#### 策略描述

被动做市策略 (Passive Market Making):
- 在买卖盘挂限价单，捕获买卖价差
- 使用订单不平衡和仓位偏移调整报价
- 自动风险管理和仓位限制

#### 策略参数

```go
spreadMultiplier  float64 // 价差倍数 (默认0.5)
orderSize         int64   // 每单大小 (默认10)
maxInventory      int64   // 最大持仓 (默认100)
inventorySkew     float64 // 仓位偏移因子 (默认0.5)
minSpread         float64 // 最小价差 (默认1.0)
orderRefreshMs    int64   // 订单刷新间隔ms (默认1000)
useOrderImbalance bool    // 是否使用订单不平衡 (默认true)
```

#### 策略逻辑

1. **指标计算**:
   - EWMA(20): 趋势跟踪
   - OrderImbalance(5档): 买卖压力
   - Spread: 当前价差
   - Volatility(20): 波动率

2. **报价计算**:
```
bid_offset = spread * spread_multiplier
ask_offset = spread * spread_multiplier

// 应用偏移
imbalance_skew = order_imbalance * 0.5
inventory_skew = (net_position / max_inventory) * inventory_skew
total_skew = imbalance_skew + inventory_skew

bid_offset += total_skew * spread * 0.3
ask_offset -= total_skew * spread * 0.3

bid_price = mid_price - bid_offset
ask_price = mid_price + ask_offset
```

3. **风险管理**:
   - 仓位限制: 不超过max_inventory
   - 价差限制: 低于min_spread不交易
   - 超限平仓: 超过限制时生成平仓信号

#### 测试结果

运行10秒，生成20个信号：
```
[Tick 1] Generated 2 signals:
  BUY ag2412 @ 7930.99, qty=10, signal=0.50, confidence=0.70
  SELL ag2412 @ 7932.99, qty=10, signal=-0.50, confidence=0.70
```

信号特点：
- 双边报价（买入+卖出）
- 价格跟随市场波动
- 每1秒刷新一次（orderRefreshMs=1000）

### 4. 策略引擎 (`pkg/strategy/engine.go`)

#### 功能

**StrategyEngine** - 策略引擎核心:
- 管理多个策略实例
- 订阅和分发市场数据
- 订阅和分发订单回报
- 处理交易信号队列
- 定时器调度
- 统计和监控

#### 架构

```
┌─────────────────────────────────────────────────────────┐
│                   Strategy Engine                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ Strategy 1  │  │ Strategy 2  │  │ Strategy N  │    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘    │
│         │                │                │            │
│         └────────────────┴────────────────┘            │
│                          │                             │
│         ┌────────────────┴────────────────┐            │
│         │                                 │            │
│    ┌────▼─────┐                  ┌────▼──────┐        │
│    │ MD       │                  │ Order     │        │
│    │ Dispatch │                  │ Dispatch  │        │
│    └────┬─────┘                  └────┬──────┘        │
│         │                             │               │
└─────────┼─────────────────────────────┼───────────────┘
          │                             │
    ┌─────▼─────┐               ┌───────▼────────┐
    │   NATS    │               │  Order Queue   │
    │  md.*     │               │  (channel)     │
    └───────────┘               └────────┬───────┘
                                         │
                                   ┌─────▼──────┐
                                   │ ORS Client │
                                   └────────────┘
```

#### 核心组件

1. **Market Data Dispatcher**:
   - 订阅NATS行情主题 "md.{symbol}"
   - 并发分发给所有运行中的策略
   - 收集策略生成的信号

2. **Order Update Dispatcher**:
   - 订阅NATS订单回报 "order.>"
   - 分发给所有策略
   - 策略自行过滤相关订单

3. **Order Processor**:
   - 从orderQueue channel读取信号
   - 转换为OrderRequest
   - 通过ORS Client发送订单

4. **Timer Loop**:
   - 定时调用所有策略的OnTimer
   - 默认5秒间隔
   - 用于策略定时任务

#### 配置

```go
type EngineConfig struct {
    ORSGatewayAddr      string        // ORS网关地址
    NATSAddr            string        // NATS地址
    OrderQueueSize      int           // 订单队列大小
    TimerInterval       time.Duration // 定时器间隔
    MaxConcurrentOrders int           // 最大并发订单
}
```

### 5. 策略演示程序 (`cmd/strategy_demo/`)

#### 功能

- 创建和配置策略引擎
- 初始化PassiveStrategy
- 模拟市场数据更新
- 实时显示交易信号
- 定期打印策略统计
- 优雅关闭

#### 运行结果

```
Strategy Engine Running - Simulating Market Data

[Tick 1] Generated 2 signals:
  BUY ag2412 @ 7930.99, qty=10, signal=0.50, confidence=0.70
  SELL ag2412 @ 7932.99, qty=10, signal=-0.50, confidence=0.70

┌────────────────────────────────────────────────────────────┐
│ Strategy: passive_1                                        │
├────────────────────────────────────────────────────────────┤
│ Running:          false                                    │
│ Position:         0                                        │
│ P&L Total:        0.00                                     │
│ Signals:          20                                       │
│ Orders:           0                                        │
│ Fills:            0                                        │
│ Rejects:          0                                        │
└────────────────────────────────────────────────────────────┘
```

## 代码统计

| 文件 | 行数 | 功能 |
|------|------|------|
| types.go | 204 | 数据类型定义 |
| strategy.go | 268 | 策略接口和基类 |
| passive_strategy.go | 330 | 被动做市策略 |
| engine.go | 360 | 策略引擎 |
| strategy_demo/main.go | 232 | 演示程序 |
| **总计** | **1,394** | **4个核心文件** |

## 技术特点

### 1. 设计模式

- **策略模式**: Strategy接口统一所有策略
- **模板方法**: BaseStrategy提供通用逻辑
- **观察者模式**: 事件驱动的市场数据和订单回报
- **生产者-消费者**: Channel-based订单队列

### 2. 并发安全

- 读写锁保护策略集合
- Goroutine并发处理回调
- Channel通信避免竞态
- Panic recovery保护

### 3. 事件驱动

```
Market Data Event → OnMarketData() → GenerateSignals() → OrderQueue
Order Update Event → OnOrderUpdate() → UpdatePosition() → UpdatePNL()
Timer Event → OnTimer() → Housekeeping()
```

### 4. 模块化设计

- 策略与引擎解耦
- 指标库独立计算
- ORS Client独立通信
- 易于扩展新策略

## 集成点

### 与指标库集成

```go
// PassiveStrategy中
func (ps *PassiveStrategy) Initialize(config *StrategyConfig) error {
    // 创建EWMA指标
    ps.Indicators.Create("ewma_20", "ewma", ewmaConfig)

    // 创建OrderImbalance指标
    ps.Indicators.Create("order_imbalance", "order_imbalance", oiConfig)
}

func (ps *PassiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
    // 更新所有指标
    ps.Indicators.UpdateAll(md)

    // 使用指标值
    oi, _ := ps.Indicators.Get("order_imbalance")
    imbalance := oi.GetValue()
}
```

### 与ORS客户端集成

```go
// StrategyEngine中
func (se *StrategyEngine) processOrders() {
    for signal := range se.orderQueue {
        req := signal.ToOrderRequest()
        resp, err := se.sendOrder(ctx, req)
    }
}
```

## 未来扩展

### 1. 其他策略实现 (Week 13-14后半段)

**AggressiveStrategy** - 主动策略:
- 追涨杀跌
- 趋势跟随
- 动量交易

**HedgingStrategy** - 对冲策略:
- Delta对冲
- 跨期对冲
- 跨品种对冲

**PairwiseArbStrategy** - 配对套利:
- 统计套利
- 协整关系
- 价差回归

### 2. 风险管理增强

- [ ] 实时VaR计算
- [ ] Sharpe Ratio跟踪
- [ ] 动态仓位管理
- [ ] 风险预算分配

### 3. 性能优化

- [ ] 指标计算批处理
- [ ] 信号去重和合并
- [ ] 订单智能路由
- [ ] 延迟监控和优化

### 4. 回测功能

- [ ] 历史数据回放
- [ ] 性能分析
- [ ] 参数优化
- [ ] 压力测试

## 下一步工作 (Week 15-16)

根据统一架构设计文档：

### Portfolio Manager (组合管理)

```go
type PortfolioManager struct {
    strategies    map[string]Strategy
    allocation    map[string]float64  // 资金分配
    totalCapital  float64
    totalPnL      float64
    totalRisk     float64
}

func (pm *PortfolioManager) RebalanceCapital()
func (pm *PortfolioManager) CalculateCorrelation()
func (pm *PortfolioManager) OptimizeAllocation()
```

### Risk Manager (风控管理)

```go
type RiskManager struct {
    positionLimits  map[string]int64
    lossLimits      map[string]float64
    alerts          []RiskAlert
}

func (rm *RiskManager) CheckGlobalLimits() bool
func (rm *RiskManager) EmergencyShutdown()
func (rm *RiskManager) GenerateRiskReport()
```

## 总结

Week 13-14完成了策略引擎的核心框架：

✅ **已完成**:
1. 策略接口和基类设计
2. PassiveStrategy完整实现
3. 策略引擎核心功能
4. 事件驱动架构
5. 与指标库集成
6. 完整演示程序

🎯 **验证通过**:
- 编译成功 ✓
- 运行演示 ✓
- 生成交易信号 ✓
- 20个信号/10秒 ✓

📊 **架构进度**:
```
✅ Week 5-6:  ORS Gateway
✅ Week 7-8:  Golang订单客户端
✅ Week 9-10: Counter Gateway
✅ Week 11-12: 指标库
✅ Week 13-14: 策略引擎 + PassiveStrategy
⏳ Week 15-16: Portfolio & Risk Manager
```

---

**里程碑**: Week 13-14 策略引擎 ✅ 完成

**下一里程碑**: Week 15-16 Portfolio & Risk管理
