# m_Active 状态转移对比：tbsrc vs QuantlinkTrader

## 概述

本文档对比分析 tbsrc 中的 `m_Active` 变量与 QuantlinkTrader 中的策略激活状态管理。

---

## tbsrc 状态变量

### 核心变量

```cpp
class ExecutionStrategy {
    bool m_Active;      // 策略是否激活
    bool m_onFlat;      // 是否平仓模式
    bool m_onCancel;    // 是否取消订单
    bool m_onExit;      // 是否退出（不可恢复）
    bool m_aggFlat;     // 是否激进平仓
};
```

### 初始化

```cpp
ExecutionStrategy::ExecutionStrategy() {
    m_Active = false;    // 默认未激活（Live 模式）
    m_onFlat = false;
    m_onCancel = false;
    m_onExit = false;
    m_aggFlat = false;
}
```

---

## tbsrc 状态转移

### 1. 激活策略 (SIGUSR1)

**信号**: `SIGUSR1`
**脚本**: `startTrade.pl`

**状态变化**:
```cpp
// main.cpp: SIGUSR1 handler
Strategy->m_onExit = false;
Strategy->m_onCancel = false;
Strategy->m_onFlat = false;
Strategy->m_sendMail = false;
Strategy->m_Active = true;          // ← 激活
Strategy->HandleSquareON();
```

**前置条件**: 无
**结果**: 策略开始发送订单

---

### 2. 停止策略 (SIGTSTP)

**信号**: `SIGTSTP`
**脚本**: `stopTrade.pl`

**状态变化**:
```cpp
// main.cpp: SIGTSTP handler
Strategy->m_onExit = true;         // 标记退出
Strategy->m_onCancel = true;       // 取消订单
Strategy->m_onFlat = true;         // 平仓模式
Strategy->HandleSquareoff();
```

**HandleSquareoff() 内部逻辑**:
```cpp
void ExecutionStrategy::HandleSquareoff() {
    // 只有当持仓为 0 且没有挂单时，才真正停止
    if (m_netpos == 0 && m_onExit &&
        m_askMap.size() == 0 && m_bidMap.size() == 0) {
        if (m_onExit && m_Active) {
            TBLOG << "Positions Closed. Strategy Exiting.." << endl;
            m_Active = false;      // ← 设置为未激活
        }
    }

    // 发送平仓订单
    // ...
}
```

**前置条件**: 无
**结果**:
1. 立即停止发送新订单 (`m_onFlat = true`)
2. 取消所有挂单 (`m_onCancel = true`)
3. **异步**平仓，当持仓为 0 时才设置 `m_Active = false`

---

### 3. 激进平仓 (Signal 37)

**信号**: `37` (自定义信号)

**状态变化**:
```cpp
Strategy->m_onExit = true;
Strategy->m_onCancel = true;
Strategy->m_onFlat = true;
Strategy->m_aggFlat = true;        // ← 激进模式
Strategy->HandleSquareoff();
```

**结果**: 穿越价差快速平仓

---

## tbsrc 状态转移图

```
┌─────────────────────────────────────────────────────────────┐
│                     初始化状态                                │
│  m_Active = false (Live 模式)                                │
│  m_onFlat = false                                           │
│  m_onCancel = false                                         │
│  m_onExit = false                                           │
└──────────────────┬──────────────────────────────────────────┘
                   │
          SIGUSR1  │  startTrade.pl
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                     激活状态                                  │
│  m_Active = true     ← 可以发送订单                          │
│  m_onFlat = false                                           │
│  m_onCancel = false                                         │
│  m_onExit = false                                           │
└──────────────────┬──────────────────────────────────────────┘
                   │
       SIGTSTP     │  stopTrade.pl
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                  平仓中状态（过渡）                           │
│  m_Active = true     ← 还是 true!                           │
│  m_onFlat = true     ← 停止新订单                           │
│  m_onCancel = true   ← 取消挂单                             │
│  m_onExit = true     ← 标记退出                             │
│  → 发送平仓订单...                                           │
└──────────────────┬──────────────────────────────────────────┘
                   │
   持仓为 0        │  HandleSquareoff() 检测
   挂单为 0        │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                   停止状态（最终）                            │
│  m_Active = false    ← 异步设置                             │
│  m_onFlat = true                                            │
│  m_onCancel = true                                          │
│  m_onExit = true                                            │
└──────────────────┬──────────────────────────────────────────┘
                   │
          SIGUSR1  │  可以重新激活
                   ▼
              回到激活状态
```

---

## QuantlinkTrader 状态变量

### 核心结构

```go
type StrategyControlState struct {
    // 运行状态（5种状态）
    RunState StrategyRunState   // Active/Paused/Flattening/Exiting/Stopped

    // 激活标志（对应 m_Active）
    Active bool                 // 对应 tbsrc: m_Active

    // 控制标志（对应 tbsrc 的 bool 标志）
    FlattenMode    bool         // 对应 tbsrc: m_onFlat
    CancelPending  bool         // 对应 tbsrc: m_onCancel
    ExitRequested  bool         // 对应 tbsrc: m_onExit
    AggressiveFlat bool         // 对应 tbsrc: m_aggFlat

    // 额外信息
    FlattenReason  FlattenReason
    FlattenTime    time.Time
    CanRecoverAt   time.Time
}

// 运行状态枚举
type StrategyRunState int
const (
    StrategyRunStateActive      // 正常运行
    StrategyRunStatePaused      // 暂停（风险触发）
    StrategyRunStateFlattening  // 平仓中
    StrategyRunStateExiting     // 退出中
    StrategyRunStateStopped     // 已停止
)
```

### 初始化

```go
func NewStrategyControlState(autoActivate bool) *StrategyControlState {
    return &StrategyControlState{
        RunState:       StrategyRunStateActive,  // 默认 Active
        Active:         autoActivate,            // Live 模式: false
        FlattenMode:    false,
        CancelPending:  false,
        ExitRequested:  false,
        AggressiveFlat: false,
    }
}
```

---

## QuantlinkTrader 状态转移

### 1. 激活策略 (SIGUSR1 / HTTP POST /activate)

**触发方式**:
- Unix 信号: `SIGUSR1`
- HTTP API: `POST /api/v1/strategy/activate`

**状态变化**:
```go
// api.go / trader.go: handleActivate / SIGUSR1 handler
baseStrat.ControlState.ExitRequested = false
baseStrat.ControlState.CancelPending = false
baseStrat.ControlState.FlattenMode = false

// 关键：重置 RunState（修复后的代码）
if baseStrat.ControlState.RunState == StrategyRunStateStopped ||
   baseStrat.ControlState.RunState == StrategyRunStateFlattening {
    baseStrat.ControlState.RunState = StrategyRunStateActive
}

baseStrat.ControlState.Activate()  // Active = true
strategy.Start()                    // 确保 RunState = Active
```

**前置条件**: 无（可以从任何状态激活）
**结果**:
- `RunState = Active`
- `Active = true`
- 策略开始发送订单

---

### 2. 停止策略 (SIGUSR2 / HTTP POST /deactivate)

**触发方式**:
- Unix 信号: `SIGUSR2`
- HTTP API: `POST /api/v1/strategy/deactivate`

**状态变化**:
```go
// api.go / trader.go: handleDeactivate / SIGUSR2 handler
baseStrat.TriggerFlatten(strategy.FlattenReasonManual, false)
baseStrat.ControlState.Deactivate()
```

**TriggerFlatten() 内部**:
```go
func (bs *BaseStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
    bs.ControlState.FlattenMode = true
    bs.ControlState.CancelPending = true
    bs.ControlState.AggressiveFlat = aggressive
    bs.ControlState.FlattenReason = reason
    bs.ControlState.FlattenTime = time.Now()
    bs.ControlState.RunState = StrategyRunStateFlattening  // ← 设置状态

    // 设置恢复时间
    if reason.CanRecover() {
        bs.ControlState.CanRecoverAt = time.Now().Add(reason.RecoveryCooldown())
    }
}
```

**Deactivate() 内部**:
```go
func (scs *StrategyControlState) Deactivate() {
    scs.Active = false  // ← 立即设置
}
```

**前置条件**: 无
**结果**:
- **立即**设置 `Active = false`
- `RunState = Flattening`
- `FlattenMode = true`
- `CancelPending = true`

---

## QuantlinkTrader 状态转移图

```
┌─────────────────────────────────────────────────────────────┐
│                     初始化状态                                │
│  RunState = Active (但不运行)                                │
│  Active = false (Live 模式)                                  │
│  FlattenMode = false                                        │
└──────────────────┬──────────────────────────────────────────┘
                   │
    SIGUSR1 或     │  POST /activate
    HTTP API       │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                     激活状态                                  │
│  RunState = Active                                          │
│  Active = true       ← 可以发送订单                          │
│  FlattenMode = false                                        │
└──────────────────┬──────────────────────────────────────────┘
                   │
    SIGUSR2 或     │  POST /deactivate
    HTTP API       │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                  平仓中状态（立即）                           │
│  RunState = Flattening   ← 立即设置                         │
│  Active = false          ← 立即设置（与 tbsrc 不同！）       │
│  FlattenMode = true      ← 停止新订单                       │
│  CancelPending = true    ← 取消挂单                         │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   │  平仓完成后（可选）
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                   停止状态（可选）                            │
│  RunState = Stopped                                         │
│  Active = false                                             │
│  FlattenMode = true                                         │
└──────────────────┬──────────────────────────────────────────┘
                   │
    SIGUSR1 或     │  POST /activate（需要重置 RunState）
    HTTP API       │
                   ▼
              回到激活状态
```

---

## 关键差异对比表

| 特性 | tbsrc | QuantlinkTrader (修复前) | QuantlinkTrader (修复后) |
|------|-------|-------------------------|-------------------------|
| **初始 m_Active** | `false` (Live) | `false` (Live) | `false` (Live) |
| **激活设置 Active** | `true` | `true` | `true` |
| **停止时 Active** | **异步** `false`<br>(持仓为0后) | **立即** `false` | **立即** `false` |
| **停止时 RunState** | 无对应变量 | `Flattening`<br>**未重置**❌ | `Flattening`<br>**会重置**✅ |
| **重新激活** | 直接设置 `m_Active=true` | ❌ 失败<br>(RunState 卡在 Flattening) | ✅ 成功<br>(重置 RunState) |
| **IsRunning() 逻辑** | `m_Active` | `Active && RunState!=Stopped` | `Active && RunState!=Stopped` |
| **平仓完成判断** | 在 HandleSquareoff 中检测 | 需要策略自己处理 | 需要策略自己处理 |

---

## 核心问题分析

### 问题：重新激活失败

**tbsrc 行为**:
```
1. 激活: m_Active = true
2. 停止: m_onFlat = true, m_Active 仍然 = true（异步）
3. 平仓完成: m_Active = false
4. 重新激活: m_Active = true ✅ （成功）
```

**QuantlinkTrader (修复前)**:
```
1. 激活: Active = true, RunState = Active
2. 停止: Active = false, RunState = Flattening
3. 重新激活:
   - Active = true ✅
   - RunState 仍然 = Flattening ❌
   - IsRunning() = Active && (RunState != Stopped)
                 = true && (Flattening != Stopped)
                 = true && true
                 = true
   - 但 IsActive() = (RunState == Active)
                   = (Flattening == Active)
                   = false ❌
   - 结果: running=true, active=false（矛盾！）
```

**QuantlinkTrader (修复后)**:
```
1. 激活: Active = true, RunState = Active
2. 停止: Active = false, RunState = Flattening
3. 重新激活:
   - 检测 RunState == Flattening
   - 重置 RunState = Active ✅
   - Active = true ✅
   - IsRunning() = true ✅
   - IsActive() = true ✅
   - 结果: running=true, active=true ✅
```

---

## 修复代码

### 修复位置

1. **API 激活处理** (`pkg/trader/api.go:143-152`)
2. **信号激活处理** (`pkg/trader/trader.go:491-500`)

### 修复代码

```go
// 重置 RunState 以便可以重新 Start
if baseStrat.ControlState.RunState == strategy.StrategyRunStateStopped ||
    baseStrat.ControlState.RunState == strategy.StrategyRunStateFlattening {
    baseStrat.ControlState.RunState = strategy.StrategyRunStateActive
}
baseStrat.ControlState.Activate()
```

---

## 设计建议

### 当前设计的优缺点

**优点**:
1. ✅ 状态更清晰（5种 RunState vs 单一 m_Active）
2. ✅ 支持更多状态（Paused, Flattening, Exiting）
3. ✅ 立即停止（`Active = false` 立即生效，tbsrc 是异步）
4. ✅ 双重控制（HTTP API + Unix 信号）

**缺点**:
1. ❌ 状态复杂度高（需要同时管理 Active 和 RunState）
2. ❌ 重新激活需要显式重置 RunState（tbsrc 不需要）
3. ❌ IsRunning() 和 IsActive() 语义不清晰

### 改进建议

#### 选项 A: 简化设计（推荐）

只使用 `RunState`，移除 `Active` 字段：

```go
type StrategyControlState struct {
    RunState StrategyRunState  // Active/Inactive/Flattening/Stopped
    // 移除 Active bool
    FlattenMode bool
    CancelPending bool
    // ...
}

func (scs *StrategyControlState) Activate() {
    scs.RunState = StrategyRunStateActive
}

func (scs *StrategyControlState) Deactivate() {
    scs.RunState = StrategyRunStateInactive  // 新增 Inactive 状态
}

func (scs *StrategyControlState) IsRunning() bool {
    return scs.RunState == StrategyRunStateActive
}
```

**优点**:
- 状态定义清晰，无二义性
- 激活/停止逻辑简单
- 无需特殊处理重新激活

#### 选项 B: 保持 tbsrc 语义

让 `Active` 主导，`RunState` 辅助：

```go
func (scs *StrategyControlState) Deactivate() {
    scs.FlattenMode = true
    scs.CancelPending = true
    // 不立即设置 Active = false
    // 等待平仓完成后才设置
}

func (bs *BaseStrategy) OnFlattenComplete() {
    if bs.ControlState.FlattenMode &&
       bs.Position.NetQty == 0 &&
       len(bs.Orders) == 0 {
        bs.ControlState.Active = false
        bs.ControlState.RunState = StrategyRunStateStopped
    }
}
```

**优点**:
- 与 tbsrc 行为完全一致
- 更接近原有逻辑

**缺点**:
- 需要异步状态管理
- 更复杂

---

## 总结

### tbsrc 的设计哲学
- **简单**: 单一 `m_Active` 标志
- **异步**: 停止是渐进的（先标记，等待平仓，再停止）
- **宽松**: 可以从任何状态恢复

### QuantlinkTrader 的设计哲学
- **精确**: 5种 RunState + Active 标志
- **同步**: 停止是立即的（立即设置 Active=false）
- **严格**: 需要显式重置状态才能重新激活

### 当前状态
✅ **已修复重新激活问题**
- 激活时检测并重置 RunState
- 保持与 Web UI 的状态一致性

### 建议
对于未来重构，建议采用**选项 A（简化设计）**，统一使用 RunState 管理所有状态，移除 Active 字段，避免状态不一致问题。
