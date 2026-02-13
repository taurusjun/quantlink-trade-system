# Phase 3 实施计划：PairwiseArbStrategy（SHM 路径）

**文档日期**: 2026-02-13
**版本**: v1.0
**相关模块**: tbsrc-golang/pkg/strategy/

---

## 背景

Phase 1 完成了 SHM 通信层（`pkg/shm/`、`pkg/connector/`），Phase 2 完成了中间层（`pkg/types/`、`pkg/instrument/`、`pkg/client/`、`pkg/execution/`、`pkg/strategy/` 接口）。

Phase 3 在此基础上实现 PairwiseArbStrategy，即 C++ `PairwiseArbStrategy` 的直接 SHM 路径等价物。

**核心目标**：Go 策略通过 Phase 1-2 的 SHM 接口直接与 hftbase ORS 交互，实现配对套利的完整下单逻辑。

---

## 架构

```
connector.Connector (Phase 1)
    ↓
client.Client (Phase 2)
    ↓ OnMDUpdate / OnORSUpdate
strategy.PairwiseArbStrategy (Phase 3)
    ├── leg1: execution.LegManager (Phase 2)  ← 被动腿（报价）
    ├── leg2: execution.LegManager (Phase 2)  ← 主动腿（对冲）
    ├── spread: SpreadTracker                  ← 价差 EWA
    └── config: PairwiseConfig                 ← 从 YAML 加载
```

---

## 文件清单（7 个新文件）

```
quantlink-trade-system/tbsrc-golang/pkg/
├── strategy/
│   ├── pairwise_arb.go          # PairwiseArbStrategy 主结构体 + 初始化
│   ├── pairwise_send_order.go   # SendOrder 核心逻辑（报价+对冲）
│   ├── pairwise_callbacks.go    # MDCallBack + ORSCallBack
│   ├── pairwise_aggressive.go   # SendAggressiveOrder + CalcPendingNetposAgg
│   ├── pairwise_price.go        # GetBidPrice/GetAskPrice (隐性订单簿)
│   └── spread_tracker.go        # EWA 价差跟踪器
└── config 扩展
    └── (扩展 trader.tbsrc.yaml，无需新文件)
```

另外增加测试文件：
```
├── strategy/
│   ├── pairwise_arb_test.go
│   ├── spread_tracker_test.go
│   └── pairwise_send_order_test.go
```

---

## 实施顺序（5 步）

```
步骤 1: SpreadTracker                    ← 无依赖，纯算法
   ↓
步骤 2: PairwiseArbStrategy 结构体       ← 依赖 Step 1 + Phase 2
   ↓
步骤 3: MDCallBack + ORSCallBack         ← 依赖 Step 2
   ↓
步骤 4: SendOrder + GetBidPrice/GetAskPrice ← 依赖 Step 3
   ↓
步骤 5: SendAggressiveOrder + 集成测试    ← 依赖 Step 4
```

---

## 各步骤详细说明

### 步骤 1：SpreadTracker (`pkg/strategy/spread_tracker.go`)

**EWA 价差跟踪器**，对应 C++ 中 `avgSpreadRatio_ori`/`avgSpreadRatio`/`currSpreadRatio` 的计算。

```go
type SpreadTracker struct {
    AvgSpreadOri float64  // EWA of spread (persisted)
    AvgSpread    float64  // AvgSpreadOri + tValue
    CurrSpread   float64  // current mid1 - mid2
    TValue       float64  // external adjustment from tvar
    Alpha        float64  // EWA decay factor (from config)
    TickSize     float64  // for AVG_SPREAD_AWAY check
    AvgSpreadAway int32   // max deviation in ticks (default 20)
    IsValid      bool     // false if spread deviates too far
    Initialized  bool     // false until first update
}
```

关键方法：
- `Update(mid1, mid2 float64) bool` — 更新 currSpread，检查 AVG_SPREAD_AWAY，更新 EWA
  - `currSpread = mid1 - mid2`
  - 参考: `PairwiseArbStrategy.cpp:496-523`
- `SetTValue(v float64)` — 更新 tValue，重算 AvgSpread
- `Seed(avgSpreadOri float64)` — 从 daily_init 文件初始化

### 步骤 2：PairwiseArbStrategy 结构体 (`pkg/strategy/pairwise_arb.go`)

```go
type PairwiseArbStrategy struct {
    // 两腿管理器（Phase 2）
    Leg1     *execution.LegManager
    Leg2     *execution.LegManager

    // 价差跟踪
    Spread   *SpreadTracker

    // 合约信息
    Inst1    *instrument.Instrument
    Inst2    *instrument.Instrument

    // 阈值
    Thold1   *types.ThresholdSet
    Thold2   *types.ThresholdSet

    // 客户端
    Client   *client.Client

    // 配置
    StrategyID  int32
    Account     string
    MaxQuoteLevel int32  // default 3

    // 状态
    Active      bool
    AggRepeat   uint32  // aggressive retry counter
    LastAggSide types.TransactionType
    LastAggTS   uint64  // nanoseconds

    // 监控
    LastMonitorTS uint64
}
```

关键方法：
- `NewPairwiseArbStrategy(cfg, client, inst1, inst2, thold1, thold2)` — 构造
- `Init(dailyInitFile string)` — 从 daily_init 加载 avgSpreadRatio_ori, 昨仓
  - 参考: `PairwiseArbStrategy.cpp:7-84`
- `SetActive(active bool)` / `IsActive() bool`

### 步骤 3：回调处理 (`pkg/strategy/pairwise_callbacks.go`)

**MDCallBack**:
```go
func (pas *PairwiseArbStrategy) MDCallBack(inst *instrument.Instrument, md *shm.MarketUpdateNew)
```
- 识别是哪个腿的行情（通过 symbol 匹配）
- 委托给对应 LegManager.MDCallBack
- 更新价差: `pas.Spread.Update(mid1, mid2)`
- 如果价差无效（AVG_SPREAD_AWAY 超限），触发 HandleSquareoff
- **仅在 leg1 行情时更新 EWA**（C++ 逻辑）
- 如果活跃，调用 `pas.SendOrder()`
- 参考: `PairwiseArbStrategy.cpp:479-569`

**ORSCallBack**:
```go
func (pas *PairwiseArbStrategy) ORSCallBack(resp *shm.ResponseMsg)
```
- 通过 orderID 判断属于哪条腿（查 ordMap1 或 ordMap2）
- Leg1 成交: 重置 AggRepeat=1, 写 TCache
- Leg2: 先调 HandleAggOrder（更新 buyAggOrder/sellAggOrder 计数器）
- 委托给对应 LegManager.ORSCallBack
- 如果活跃且有未对冲头寸，调用 SendAggressiveOrder
- 参考: `PairwiseArbStrategy.cpp:428-477`

**HandleAggOrder**: 在 TRADE_CONFIRM/CANCEL_CONFIRM/REJECT 时，减少 buyAggOrder/sellAggOrder
- 参考: `PairwiseArbStrategy.cpp:402-426`

### 步骤 4：SendOrder + 价格逻辑

**SendOrder** (`pkg/strategy/pairwise_send_order.go`):

核心下单逻辑，每次行情更新时调用。参考: `PairwiseArbStrategy.cpp:146-385`

```
Phase 1: SetThresholds (已有 Phase 2 实现)

Phase 2: 撤销所有 CROSS/MATCH 订单（两腿）

Phase 3: 撤销偏离均值的 Leg1 订单
  - Bid: if (ourBidPx - leg2.bid[0]) > avgSpread - tholdBidRemove → cancel
  - Ask: if (ourAskPx - leg2.ask[0]) < avgSpread + tholdAskRemove → cancel

Phase 4: 零价格保护

Phase 5: 多档报价循环 (level 0..MaxQuoteLevel-1)
  - ASK: if ShortSpread > avgSpread + tholdAskPlace → SendAskOrder2
  - BID: if LongSpread < avgSpread - tholdBidPlace → SendBidOrder2

Phase 6: Leg2 对冲
  - net_exposure = leg1.netpos_pass + leg2.netpos_agg + pendingAgg2
  - if > 0: CROSS sell on leg2
  - if < 0: CROSS buy on leg2
```

**GetBidPrice_first / GetAskPrice_first** (`pkg/strategy/pairwise_price.go`):

隐性订单簿价格改善逻辑。参考: `PairwiseArbStrategy.cpp:802-840`

```
IF useInvisibleBook AND level > 0 AND price < prevLevelPrice - tickSize:
    bidInv = price - leg2.bid[0] + tickSize
    IF bidInv <= avgSpread - BEGIN_PLACE:
        IF quantAhead at this price > lotSize:
            price += tickSize (improve by 1 tick)
```

### 步骤 5：SendAggressiveOrder + 集成测试

**SendAggressiveOrder** (`pkg/strategy/pairwise_aggressive.go`):

对冲腿的激进下单逻辑。参考: `PairwiseArbStrategy.cpp:701-800`

```go
func (pas *PairwiseArbStrategy) SendAggressiveOrder()
```

1. 计算 net_exposure = leg1.netpos_pass + leg2.netpos_agg + pendingAgg2
2. 如果 net_exposure > 0（需要卖 leg2）:
   - 首次或间隔 > 500ms: 在 leg2.bid[0] 挂卖单 (CROSS)
   - 重试 < 3 次: bid[0] - tickSize * repeat
   - 重试 == 3: bid[0] - tickSize * SLOP（大幅穿越）
   - 重试 > 3: HandleSquareoff（停策略）
3. 镜像逻辑处理 net_exposure < 0

**CalcPendingNetposAgg**: 遍历 leg2 的 ordMap，统计所有 CROSS/MATCH 订单的净挂单量。

**HandleSquareoff**: 设置两腿 OnExit/OnCancel/OnFlat，撤销全部订单，停策略，保存 daily_init。
- 参考: `PairwiseArbStrategy.cpp:586-626`

---

## 配置扩展

在 `config/trader.tbsrc.yaml` 的 strategy 段增加：

```yaml
strategy:
  # ... 已有字段 ...
  max_quote_level: 3
  use_invisible_book: false
  daily_init_file: "../data/daily_init.92201"
```

---

## 验证标准

| 步骤 | 验证 |
|------|------|
| 1 | SpreadTracker EWA 计算正确；AVG_SPREAD_AWAY 保护工作；seed/tValue 正确 |
| 2 | 构造正确初始化两个 LegManager；daily_init 加载昨仓和 avgSpreadOri |
| 3 | MDCallBack 正确路由两腿；EWA 仅在 leg1 更新时刷新；ORSCallBack 正确路由 |
| 4 | SendOrder 多档报价逻辑与 C++ 一致；撤单阈值正确；隐性订单簿改善价格正确 |
| 5 | SendAggressiveOrder 重试阶梯正确；超限触发 HandleSquareoff；全流程集成测试通过 |

最终验证：
```bash
go test ./pkg/... -v -count=1
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
```

---

## Phase 3 不包含的内容（推迟到 Phase 4+）

| 功能 | 推迟原因 |
|------|---------|
| tValue/tvar SHM 实际加载 | 需要真实 SHM 环境 |
| tcache 位置共享 | 需要真实 SHM 环境 |
| daily_init 文件写入 | 需要文件系统集成 |
| 实盘 E2E 测试 | 需要 ORS 运行环境 |
| Indicator 系统 | 新系统 golang/ 已有完整实现 |
| 回测集成 | 新系统 golang/ 已有 backtest/ 包 |

---

## 关键 C++ 参考文件

| Go 文件 | C++ 参考 |
|---------|---------|
| strategy/spread_tracker.go | PairwiseArbStrategy.cpp:496-523 |
| strategy/pairwise_arb.go | PairwiseArbStrategy.cpp:7-84, include/PairwiseArbStrategy.h |
| strategy/pairwise_callbacks.go | PairwiseArbStrategy.cpp:428-569 |
| strategy/pairwise_send_order.go | PairwiseArbStrategy.cpp:146-385 |
| strategy/pairwise_aggressive.go | PairwiseArbStrategy.cpp:701-800 |
| strategy/pairwise_price.go | PairwiseArbStrategy.cpp:802-883 |

---

**最后更新**: 2026-02-13 21:20
