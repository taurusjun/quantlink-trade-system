# RunState 对应关系：tbsrc vs QuantlinkTrader

## 核心结论

**tbsrc 中没有 RunState 这个单独的变量！**

tbsrc 使用**多个布尔标志的组合**来表示策略的不同运行状态。

QuantlinkTrader 的 `RunState` 枚举是对这些布尔标志组合的**抽象和枚举化**。

---

## tbsrc 的状态变量

### ExecutionStrategy.h 中的状态标志

```cpp
class ExecutionStrategy {
    // 核心控制标志
    bool m_Active;          // 策略是否激活（可以发送订单）
    bool m_onFlat;          // 是否进入平仓模式
    bool m_onCancel;        // 是否取消所有订单
    bool m_onExit;          // 是否退出（不可恢复）
    bool m_aggFlat;         // 是否激进平仓

    // 风险触发标志
    bool m_onStopLoss;      // 止损触发
    bool m_onMaxPx;         // 最大价格触发
    bool m_onNewsFlat;      // 新闻事件触发平仓
    bool m_onTimeSqOff;     // 时间触发平仓

    // 其他
    bool m_sendMail;        // 是否发送邮件
    bool m_optionStrategy;  // 是否期权策略
    // ... 更多标志
};
```

**设计思路**：使用多个独立的布尔标志，通过**组合**来表示不同的运行状态。

---

## tbsrc 的状态组合

### 状态 1: 正常运行 (Active)

```cpp
m_Active = true
m_onFlat = false
m_onCancel = false
m_onExit = false
```

**对应 QuantlinkTrader**: `RunState = StrategyRunStateActive`

---

### 状态 2: 平仓中 (Flattening)

```cpp
m_Active = true       // ← 注意：还是 true!
m_onFlat = true       // ← 停止新订单
m_onCancel = true     // ← 取消挂单
m_onExit = true       // ← 标记退出
```

**对应 QuantlinkTrader**: `RunState = StrategyRunStateFlattening`

**关键差异**：
- tbsrc: `m_Active` 在平仓过程中仍然是 `true`
- QuantlinkTrader: `Active` 立即变为 `false`

---

### 状态 3: 已停止 (Stopped)

```cpp
m_Active = false      // ← 持仓为0后才设置
m_onFlat = true
m_onCancel = true
m_onExit = true
```

**对应 QuantlinkTrader**: `RunState = StrategyRunStateStopped`

---

### 状态 4: 风险暂停 (Paused)

**tbsrc 实现**：
```cpp
// 止损触发
m_Active = true       // 策略还在运行
m_onStopLoss = true   // 但因为止损暂停
m_onFlat = true       // 进入平仓模式

// 或者价格触发
m_onMaxPx = true
m_onFlat = true
```

**对应 QuantlinkTrader**: `RunState = StrategyRunStatePaused`

**注**：tbsrc 的风险暂停通常也会触发平仓，与 Flattening 类似。

---

### 状态 5: 退出中 (Exiting)

**tbsrc 实现**：
```cpp
m_onExit = true       // 标记退出（不可恢复）
m_onFlat = true
m_onCancel = true
m_Active = true       // 还在平仓过程中
```

**对应 QuantlinkTrader**: `RunState = StrategyRunStateExiting`

---

## 完整映射表

| QuantlinkTrader RunState | tbsrc 布尔标志组合 | 含义 |
|--------------------------|-------------------|------|
| **StrategyRunStateActive** | `m_Active=true`<br>`m_onFlat=false`<br>`m_onExit=false` | 正常运行，可以发送订单 |
| **StrategyRunStatePaused** | `m_Active=true`<br>`m_onStopLoss=true` 或<br>`m_onMaxPx=true` | 风险触发暂停（通常也会平仓） |
| **StrategyRunStateFlattening** | `m_Active=true` ⚠️<br>`m_onFlat=true`<br>`m_onCancel=true`<br>`m_onExit=true` | 平仓中（tbsrc 的 m_Active 还是 true） |
| **StrategyRunStateExiting** | `m_Active=true` ⚠️<br>`m_onExit=true`<br>`m_onFlat=true`<br>不可恢复 | 退出中（不可恢复的平仓） |
| **StrategyRunStateStopped** | `m_Active=false`<br>`m_onExit=true` | 已停止，持仓为 0 |

---

## 关键差异

### 1. 单一枚举 vs 多个布尔标志

**tbsrc**:
```cpp
// 需要检查多个标志
if (m_Active && !m_onFlat && !m_onExit) {
    // 可以发送订单
}

if (m_onFlat) {
    // 进入平仓逻辑
}
```

**QuantlinkTrader**:
```go
// 简单的枚举检查
switch RunState {
case StrategyRunStateActive:
    // 可以发送订单
case StrategyRunStateFlattening:
    // 进入平仓逻辑
}
```

**优点**：
- ✅ 更清晰、更易读
- ✅ 状态互斥（不会出现矛盾的标志组合）
- ✅ 类型安全

---

### 2. 平仓中的 Active 状态

**tbsrc**: 平仓过程中 `m_Active` 保持 `true`
```cpp
// SIGTSTP 信号处理
Strategy->m_onExit = true;
Strategy->m_onCancel = true;
Strategy->m_onFlat = true;
// m_Active 还是 true!

// HandleSquareoff() 中才设置
if (m_netpos == 0 && m_askMap.size() == 0) {
    m_Active = false;  // 异步设置
}
```

**QuantlinkTrader**: 平仓时立即设置 `Active = false`
```go
// TriggerFlatten()
baseStrat.ControlState.FlattenMode = true
baseStrat.ControlState.RunState = StrategyRunStateFlattening
baseStrat.ControlState.Deactivate()  // 立即 Active = false
```

**结果**：
- tbsrc: `m_Active` 是异步的，需要等待平仓完成
- QuantlinkTrader: `Active` 是同步的，立即生效

---

### 3. IsRunning() 的实现

**tbsrc**:
```cpp
// 直接检查 m_Active
bool IsRunning() {
    return m_Active;
}
```

**QuantlinkTrader**:
```go
// 需要同时检查 Active 和 RunState
func (bs *BaseStrategy) IsRunning() bool {
    return bs.ControlState.IsActivated() &&
           bs.ControlState.RunState != StrategyRunStateStopped
}
```

**问题**：两个变量可能不一致！
- `Active = true, RunState = Flattening` → IsRunning = true
- 但 `IsActive()` 检查 `RunState == Active` → false

---

## 为什么 QuantlinkTrader 引入了 RunState？

### tbsrc 的问题

1. **状态不清晰**：需要检查多个布尔标志组合
   ```cpp
   // 这是什么状态？
   m_Active = true
   m_onFlat = true
   m_onCancel = false
   m_onExit = false
   ```

2. **容易出现矛盾状态**：
   ```cpp
   // 可能的矛盾
   m_Active = false
   m_onFlat = false  // 既不活动又不平仓？
   ```

3. **缺乏类型安全**：编译器不会检查标志组合是否合法

4. **扩展困难**：添加新状态需要引入更多布尔标志

### QuantlinkTrader 的改进

1. **状态明确**：5 种清晰的枚举状态
2. **互斥性**：同一时间只能处于一种状态
3. **类型安全**：编译器检查
4. **易于扩展**：添加新状态只需扩展枚举

---

## 当前的问题

### 双重状态带来的复杂性

QuantlinkTrader 同时维护了：
1. `Active bool` - 继承自 tbsrc 的 `m_Active`
2. `RunState` - 新引入的枚举状态

**导致问题**：
- 两个变量需要保持同步
- `Active` 和 `RunState` 可能不一致
- 重新激活时需要同时重置两个变量

### 你遇到的 Bug

```go
// 停止后
Active = false
RunState = Flattening

// 重新激活（修复前）
Active = true           // ✅ 已设置
RunState = Flattening   // ❌ 忘记重置

// IsRunning() = true (因为 Active=true && RunState!=Stopped)
// IsActive() = false (因为 RunState != Active)
// 矛盾！
```

---

## 设计建议

### 选项 1：移除 Active，只用 RunState（推荐）

```go
type StrategyControlState struct {
    RunState StrategyRunState  // 唯一的状态变量
    // 移除 Active bool

    FlattenMode    bool  // 辅助标志
    CancelPending  bool
    ExitRequested  bool
}

func (scs *StrategyControlState) Activate() {
    scs.RunState = StrategyRunStateActive
}

func (scs *StrategyControlState) Deactivate() {
    scs.RunState = StrategyRunStateFlattening
}

func (scs *StrategyControlState) IsRunning() bool {
    return scs.RunState == StrategyRunStateActive ||
           scs.RunState == StrategyRunStatePaused
}
```

**优点**：
- 单一状态来源
- 不会出现不一致
- 更简单清晰

---

### 选项 2：Active 主导，RunState 细化（当前）

保持 `Active` 作为主要标志，`RunState` 提供更细的状态：

```go
func (scs *StrategyControlState) IsRunning() bool {
    return scs.Active &&
           scs.RunState != StrategyRunStateStopped
}
```

**优点**：
- 与 tbsrc 的 `m_Active` 语义接近
- 保留了细粒度的 RunState

**缺点**：
- 需要维护两个变量的一致性
- 激活时需要显式重置 RunState

---

## 总结

### 直接回答你的问题

**Q: RunState 对应 tbsrc 中的哪个变量？**

**A: tbsrc 中没有单独的 RunState 变量！**

tbsrc 使用多个布尔标志的组合：
- `m_Active` + `m_onFlat` + `m_onCancel` + `m_onExit` + ...

QuantlinkTrader 的 `RunState` 枚举是对这些**布尔标志组合**的抽象。

---

### 映射关系

| QuantlinkTrader | tbsrc 等价组合 |
|-----------------|---------------|
| `RunState = Active` | `m_Active=true && m_onFlat=false` |
| `RunState = Flattening` | `m_Active=true && m_onFlat=true` |
| `RunState = Stopped` | `m_Active=false` |
| `RunState = Paused` | `m_Active=true && m_onStopLoss=true` |
| `RunState = Exiting` | `m_Active=true && m_onExit=true` (不可恢复) |

---

### 设计演化

```
tbsrc:           多个布尔标志 (灵活但容易混乱)
                       ↓
QuantlinkTrader: RunState 枚举 + Active 标志 (清晰但需要同步)
                       ↓
未来建议:        只用 RunState 枚举 (最简单清晰)
```
