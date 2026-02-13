# Phase 2 实施计划：CommonClient + ExecutionStrategy 基础框架

**文档日期**: 2026-02-13
**版本**: v1.0
**相关模块**: tbsrc-golang/pkg/types, instrument, client, execution, strategy

---

## 概述

Phase 1 已完成 SHM 通信层（`pkg/shm/`、`pkg/connector/`、`pkg/config/`），21 个测试通过，交叉编译验证通过。

Phase 2 构建 Connector 与具体策略之间的中间层，对应 C++ 中的 `CommonClient`、`ExecutionStrategy` 基类、`ExtraStrategy`。这是 Phase 3（PairwiseArbStrategy 具体策略）的前提。

**核心目标**：Go 策略能通过相同的 SHM 接口与 hftbase ORS 交互，订单管理、持仓跟踪、PNL 计算与 C++ 逐行一致。

---

## 架构差异说明

C++ 使用继承链：`ExecutionStrategy` → `ExtraStrategy` → `PairwiseArbStrategy`

Go 使用组合：
```
connector.Connector  (Phase 1, 已完成)
    ↓
client.Client        (对应 CommonClient：MD/ORS 路由，RequestMsg 构造)
    ↓
execution.LegManager (对应 ExtraStrategy：单腿的订单+持仓+PNL)
    ├── execution.ExecutionState  (持仓、PNL、计数器)
    └── execution.OrderManager    (OrderMap、PriceMap、下单/撤单)
    ↓
strategy.Strategy    (接口，Phase 3 实现具体策略)
```

---

## 文件清单（14 个新文件）

```
quantlink-trade-system/tbsrc-golang/pkg/
├── types/                          # 新包：枚举和数据结构
│   ├── enums.go                    # OrderStatus, OrderHitType, TransactionType, TypeOfOrder
│   ├── order_stats.go              # OrderStats 结构体
│   └── threshold_set.go            # ThresholdSet 结构体 + 默认值
├── instrument/                     # 新包：行情簿管理
│   └── instrument.go               # Instrument 结构体、UpdateFromMD、MSW/MID 价格计算
├── client/                         # 新包：对应 CommonClient
│   └── client.go                   # MD/ORS 路由、RequestMsg 构造、orderID-strategy 映射
├── execution/                      # 新包：执行引擎基础
│   ├── state.go                    # ExecutionState：持仓、PNL、计数器、Reset()
│   ├── pnl.go                      # CalculatePNL()
│   ├── threshold.go                # SetThresholds()、SetLinearThresholds()
│   ├── order_manager.go            # OrderManager：OrderMap/PriceMap、SendNewOrder/Modify/Cancel
│   ├── ors_callback.go             # ORS 回调处理：成交/撤单/改单确认、拒绝
│   ├── squareoff.go                # CheckSquareoff()、HandleSquareoff()
│   └── leg_manager.go              # LegManager：组合 State + OrderManager（对应 ExtraStrategy）
└── strategy/                       # 新包：策略接口
    └── strategy.go                 # Strategy 接口定义
```

---

## 实施顺序（8 步）

### 步骤 1：基础类型 `pkg/types/`

**enums.go** — 所有枚举，值与 C++ 完全一致：

| Go 类型 | C++ 来源 | 值 |
|---------|---------|---|
| `OrderStatus` | `ExecutionStrategyStructs.h` | NEW_ORDER=0 ... INIT=10 |
| `OrderHitType` | `ExecutionStrategyStructs.h` | STANDARD=0, IMPROVE=1, CROSS=2, DETECT=3, MATCH=4 |
| `TransactionType` | `ORSBase.h` | Buy=1, Sell=2 |
| `TypeOfOrder` | `ORSBase.h` | Quote=0, PHedge=1, AHedge=2 |

**order_stats.go** — `OrderStats` 结构体，字段完全对应 C++ `OrderStats`

**threshold_set.go** — `ThresholdSet` 结构体 + `NewThresholdSet()` 默认值

### 步骤 2：Instrument `pkg/instrument/`

简化版 Instrument，关键方法：
- `UpdateFromMD(md *shm.MarketUpdateNew)` — 从 SHM MarketUpdateNew 更新 20 档行情簿
- `MidPrice() float64` — `(BidPx[0] + AskPx[0]) / 2`
- `MSWPrice() float64` — 市场量加权价

参考：`tbsrc/common/include/Instrument.h`

### 步骤 3：ExecutionState `pkg/execution/state.go`

持仓、PNL、计数器、`Reset()` 方法
参考：`ExecutionStrategy.h:85-308`，`ExecutionStrategy.cpp:276-396`

### 步骤 4：PNL + Threshold

**pnl.go** — `CalculatePNL()` 参考 `ExecutionStrategy.cpp:2124-2148`
**threshold.go** — `SetThresholds()` / `SetLinearThresholds()` 参考 `ExecutionStrategy.cpp:500-689`

### 步骤 5：Client `pkg/client/`

MD/ORS 路由、RequestMsg 构造、orderID-strategy 映射
参考：`CommonClient.cpp`

### 步骤 6：OrderManager + ORS 回调

订单管理（重复价格检查、状态跟踪）+ ORS 响应状态机
参考：`ExtraStrategy.cpp:201-485`，`ExecutionStrategy.cpp:951-2122`

### 步骤 7：LegManager + Squareoff

组合 State + OrderManager，发单逻辑 + 平仓逻辑
参考：`ExtraStrategy.cpp:33-623`

### 步骤 8：Strategy 接口 + Config 扩展

Strategy 接口定义，扩展 YAML 配置

---

## 关键 C++ 参考文件

| Go 文件 | C++ 参考 |
|---------|---------|
| types/enums.go | `ExecutionStrategyStructs.h`, `ORSBase.h` |
| types/order_stats.go | `ExecutionStrategyStructs.h:44-68` |
| types/threshold_set.go | `TradeBotUtils.h:237-504` |
| instrument/instrument.go | `tbsrc/common/include/Instrument.h` |
| client/client.go | `tbsrc/main/CommonClient.cpp` |
| execution/state.go | `ExecutionStrategy.h:85-308` |
| execution/pnl.go | `ExecutionStrategy.cpp:2124-2148` |
| execution/threshold.go | `ExecutionStrategy.cpp:500-689` |
| execution/order_manager.go | `ExtraStrategy.cpp:201-485` |
| execution/ors_callback.go | `ExecutionStrategy.cpp:951-1154, 1888-2122` |
| execution/squareoff.go | `ExtraStrategy.cpp:542-623` |
| execution/leg_manager.go | `ExtraStrategy.cpp:33-199, 487-540` |

---

## 验证标准

| 步骤 | 验证 |
|------|------|
| 1 | 枚举值与 C++ 一致，ThresholdSet 默认值正确 |
| 2 | UpdateFromMD 正确更新 20 档行情，MSW/MID 价格计算正确 |
| 3 | Reset() 归零所有字段 |
| 4 | CalculatePNL 多头/空头/平仓场景与 C++ 公式一致 |
| 5 | Client MD 路由按 symbol 分发，ORS 路由按 orderID 分发 |
| 6 | 订单状态机完整周期 |
| 7 | LegManager 集成测试 |
| 8 | 配置加载通过，`go test ./pkg/...` 全部通过 |

---

**最后更新**: 2026-02-13 10:00
