# 策略激活机制对比：QuantlinkTrader vs tbsrc TradeBot

**日期**: 2026-01-22

---

## 概述

对比 tbsrc TradeBot 和 QuantlinkTrader 的策略激活和控制机制。

---

## tbsrc TradeBot 激活机制

### 1. 启动时的初始状态

**代码** (`ExecutionStrategy.cpp:378-380`):
```cpp
if (m_configParams->m_modeType == ModeType_Sim)
    m_Active = true;   // Sim 模式：自动激活
else
    m_Active = false;  // Live 模式：手动激活
```

**初始状态**:
- **Simulation 模式**: `m_Active = true` - 启动即激活，立即开始交易
- **Live 模式**: `m_Active = false` - 启动后**不激活**，等待手动激活

### 2. 通过 Unix 信号控制

**信号处理** (`main.cpp:132-149`):
```cpp
// 信号处理器
void sqoff(int sig) {
    if (sig == SIGTSTP || (sig == SIGTERM && isSymbol)) {
        // 停止并平仓（Squareoff）
        Strategy->m_onExit = true;
        Strategy->m_onCancel = true;
        Strategy->m_onFlat = true;
        Strategy->HandleSquareoff();
    }
    else if (sig == SIGUSR1 || (sig == SIGRTMIN && isSymbol)) {
        // 激活策略（Start Trading）
        Strategy->m_onExit = false;
        Strategy->m_onCancel = false;
        Strategy->m_onFlat = false;
        Strategy->m_sendMail = false;
        Strategy->m_Active = true;     // ← 激活！
        Strategy->HandleSquareON();
        TBLOG << "Strategy is active now." << endl;
    }
}
```

**支持的信号**:
| 信号 | 作用 | 说明 |
|------|------|------|
| `SIGUSR1` | 激活策略 | 设置 `m_Active = true`，开始交易 |
| `SIGTSTP` | 停止并平仓 | 设置 `m_Active = false`，取消订单，平仓 |
| `SIGTERM` | 停止并平仓 | 同 SIGTSTP |
| `SIGUSR2` | 其他控制 | - |

### 3. 外部控制脚本

#### 启动交易 (`startTrade.pl`)

```perl
#!/usr/bin/perl
my $strategy_id = $ARGV[0];

# 1. 从 lock 文件读取进程 PID
my $process_id = `cat $LOCK_DIR/lock.$strategy_id | awk '{print \$1}'`;

# 2. 找到策略进程的实际 PID
my @execs = `ps -o pid --ppid $process_id`;
my @execs2 = `ps -o pid --ppid $execs[0]`;
my $pid = $execs2[0];

# 3. 发送 SIGUSR1 信号激活策略
`kill -SIGUSR1 $pid`;
```

**使用**:
```bash
# 激活策略 92201
perl startTrade.pl 92201

# 或批量激活所有策略
pkill -SIGUSR1 TradeBot
```

#### 停止交易 (`stopTrade.pl`)

```perl
#!/usr/bin/perl
my $strategy_id = $ARGV[0];

# 找到 PID
my $process_id = `cat $LOCK_DIR/lock.$strategy_id | awk '{print \$1}'`;
my @execs = `ps -o pid --ppid $process_id`;
my @execs2 = `ps -o pid --ppid $execs[0]`;
my $pid = $execs2[0];

# 发送 SIGTSTP 信号停止并平仓
`kill -SIGTSTP $pid`;
```

**使用**:
```bash
# 停止策略 92201
perl stopTrade.pl 92201

# 或批量停止所有策略
pkill -SIGTSTP TradeBot
```

### 4. 完整的控制流程

```
┌─────────────────────────────────────────────────────────────┐
│ TradeBot 启动（Live 模式）                                   │
│ m_Active = false                                             │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ （策略不交易，但接收市场数据）
                 │
                 ▼
        ┌────────────────────┐
        │ 等待激活信号        │
        └────────────────────┘
                 │
                 │ perl startTrade.pl 92201
                 │ （发送 SIGUSR1 信号）
                 ▼
        ┌────────────────────┐
        │ 信号处理器          │
        │ m_Active = true     │
        └────────────────────┘
                 │
                 ▼
        ┌────────────────────┐
        │ 策略开始交易        │
        │ 生成信号、下单      │
        └────────────────────┘
                 │
                 │ （交易进行中...）
                 │
                 │ perl stopTrade.pl 92201
                 │ （发送 SIGTSTP 信号）
                 ▼
        ┌────────────────────┐
        │ 信号处理器          │
        │ m_onFlat = true     │
        │ m_Active = false    │
        └────────────────────┘
                 │
                 ▼
        ┌────────────────────┐
        │ HandleSquareoff()   │
        │ - 取消所有订单      │
        │ - 平掉所有仓位      │
        │ - 停止交易          │
        └────────────────────┘
```

### 5. m_Active 的作用

**在策略代码中的检查** (`ExecutionStrategy.cpp`):
```cpp
void ExecutionStrategy::OnTimerUpdate() {
    // 检查策略是否激活
    if (!m_Active) {
        return;  // 不激活时不生成信号
    }

    // 检查风险限制
    CheckSquareoff();

    // 生成交易信号
    GenerateSignals();
}

void ExecutionStrategy::GenerateSignals() {
    if (!m_Active) {
        return;  // 不激活时不下单
    }

    // ... 生成交易信号
}
```

**关键点**:
- ✅ `m_Active = false` 时，策略**不生成信号**，**不下单**
- ✅ 但策略**仍在运行**，仍接收市场数据
- ✅ 可以随时通过信号重新激活

---

## QuantlinkTrader 当前机制

### 1. 启动时的初始状态

**代码** (`pkg/trader/trader.go`):
```go
func (t *Trader) Start() error {
    // ... 启动各组件 ...

    // 启动策略（如果 auto_start 或在交易时段内）
    if t.Config.Session.AutoStart || t.SessionMgr.IsInSession() {
        if err := t.Strategy.Start(); err != nil {
            return err
        }
        log.Println("[Trader] ✓ Strategy started")
    }

    // ... 启动会话管理器 ...
}
```

**初始状态**:
- **所有模式**: 如果 `auto_start = true` 或在交易时段内，**自动激活**
- **无法手动控制**: 启动后立即激活，无外部控制

### 2. 当前的策略状态控制

**BaseStrategy** (`pkg/strategy/strategy.go`):
```go
type BaseStrategy struct {
    ControlState *StrategyControlState  // 状态控制
    // ...
}

type StrategyControlState struct {
    Active         bool  // 对应 tbsrc m_Active
    FlattenMode    bool  // 对应 tbsrc m_onFlat
    CancelPending  bool  // 对应 tbsrc m_onCancel
    ExitRequested  bool  // 对应 tbsrc m_onExit
    // ...
}

func (bs *BaseStrategy) Activate() {
    bs.ControlState.Active = true
}

func (bs *BaseStrategy) Deactivate() {
    bs.ControlState.Active = false
}
```

**问题**:
- ✅ 有 `Active` 标志（对应 `m_Active`）
- ✅ 有 `Activate()` / `Deactivate()` 方法
- ❌ **没有外部控制接口**（无法从外部激活/停止）
- ❌ **没有信号处理**

### 3. 会话管理器

**SessionManager** (`pkg/trader/session.go`):
```go
func (t *Trader) runSessionManager() {
    ticker := time.NewTicker(1 * time.Second)

    for t.IsRunning() {
        <-ticker.C

        inSession := t.SessionMgr.IsInSession()
        strategyRunning := t.Strategy.IsRunning()

        // 自动启动（在交易时段内）
        if inSession && !strategyRunning && t.Config.Session.AutoStart {
            t.Strategy.Start()
        }

        // 自动停止（交易时段外）
        if !inSession && strategyRunning && t.Config.Session.AutoStop {
            t.Strategy.Stop()
        }
    }
}
```

**特点**:
- ✅ 根据交易时段自动启停
- ❌ **无法手动控制**（只能通过时段自动控制）

---

## 对比总结

### 核心差异

| 方面 | tbsrc TradeBot | QuantlinkTrader | 对齐状态 |
|------|----------------|-----------------|----------|
| **启动时状态（Live）** | `m_Active = false` 等待激活 | 自动激活 | ❌ **不一致** |
| **启动时状态（Sim）** | `m_Active = true` 自动激活 | 自动激活 | ✅ 一致 |
| **外部控制** | Unix 信号（SIGUSR1/SIGTSTP） | 无 | ❌ **缺失** |
| **控制脚本** | startTrade.pl / stopTrade.pl | 无 | ❌ **缺失** |
| **手动激活** | ✅ 支持 | ❌ 不支持 | ❌ **缺失** |
| **手动停止** | ✅ 支持（保持进程运行） | ❌ 只能退出整个程序 | ❌ **缺失** |
| **批量控制** | ✅ `pkill -SIGUSR1 TradeBot` | ❌ 无 | ❌ **缺失** |
| **时段自动控制** | ❌ 无 | ✅ 有 | ➕ **增强** |

### 控制流程对比

#### tbsrc TradeBot（Live 模式）

```
启动 TradeBot
    ↓
m_Active = false（不交易）
    ↓
接收市场数据（不生成信号）
    ↓
[手动] perl startTrade.pl 92201
    ↓
SIGUSR1 信号 → m_Active = true
    ↓
开始交易（生成信号、下单）
    ↓
[手动] perl stopTrade.pl 92201
    ↓
SIGTSTP 信号 → m_onFlat = true, m_Active = false
    ↓
停止交易（取消订单、平仓）
    ↓
回到等待状态（仍在运行）
    ↓
[可选] 再次激活...
```

#### QuantlinkTrader（当前）

```
启动 QuantlinkTrader
    ↓
检查 auto_start 或交易时段
    ↓
自动激活（Strategy.Start()）
    ↓
开始交易（生成信号、下单）
    ↓
[只能] Ctrl+C 退出整个程序
    ↓
程序终止（无法重新激活）
```

**问题**:
- ❌ 无法在运行时手动启动/停止交易
- ❌ 停止交易必须退出整个程序
- ❌ 无法批量控制多个策略

---

## 需要改进的地方

### 1. 添加外部控制接口

#### 方案 A: Unix 信号（与 tbsrc 一致）

**优点**:
- ✅ 与 tbsrc 完全一致
- ✅ 简单、直接
- ✅ 支持批量控制

**实现**:
```go
// pkg/trader/signals.go
func (t *Trader) setupSignalHandlers() {
    sigChan := make(chan os.Signal, 1)

    // 监听激活/停止信号
    signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGUSR2)

    go func() {
        for sig := range sigChan {
            switch sig {
            case syscall.SIGUSR1:
                // 激活策略
                t.activateStrategy()
            case syscall.SIGUSR2:
                // 停止策略（平仓）
                t.deactivateStrategy()
            }
        }
    }()
}

func (t *Trader) activateStrategy() {
    log.Println("[Trader] Received SIGUSR1: Activating strategy")
    t.Strategy.Activate()
    t.Strategy.Start()
}

func (t *Trader) deactivateStrategy() {
    log.Println("[Trader] Received SIGUSR2: Deactivating strategy (squareoff)")
    t.Strategy.TriggerFlatten(FlattenReasonManual, false)
}
```

**使用**:
```bash
# 激活策略
kill -SIGUSR1 <PID>

# 停止策略
kill -SIGUSR2 <PID>

# 批量激活所有策略
pkill -SIGUSR1 QuantlinkTrader
```

#### 方案 B: HTTP REST API

**优点**:
- ✅ 更现代
- ✅ 支持远程控制
- ✅ 返回详细状态

**实现**:
```go
// pkg/trader/api.go
type TraderAPI struct {
    trader *Trader
}

func (api *TraderAPI) StartHTTPServer(addr string) {
    http.HandleFunc("/api/v1/strategy/activate", api.handleActivate)
    http.HandleFunc("/api/v1/strategy/deactivate", api.handleDeactivate)
    http.HandleFunc("/api/v1/strategy/status", api.handleStatus)

    log.Printf("[API] HTTP server listening on %s", addr)
    http.ListenAndServe(addr, nil)
}

func (api *TraderAPI) handleActivate(w http.ResponseWriter, r *http.Request) {
    api.trader.Strategy.Activate()
    api.trader.Strategy.Start()
    json.NewEncoder(w).Encode(map[string]string{"status": "activated"})
}

func (api *TraderAPI) handleDeactivate(w http.ResponseWriter, r *http.Request) {
    api.trader.Strategy.TriggerFlatten(FlattenReasonManual, false)
    json.NewEncoder(w).Encode(map[string]string{"status": "deactivated"})
}
```

**使用**:
```bash
# 激活策略
curl -X POST http://localhost:8080/api/v1/strategy/activate

# 停止策略
curl -X POST http://localhost:8080/api/v1/strategy/deactivate

# 查看状态
curl http://localhost:8080/api/v1/strategy/status
```

#### 方案 C: 命令行工具

**优点**:
- ✅ 类似 tbsrc 脚本
- ✅ 易用

**实现**:
```bash
# trader-ctl 命令行工具
./trader-ctl --strategy-id 92201 --action start
./trader-ctl --strategy-id 92201 --action stop
./trader-ctl --strategy-id 92201 --action status
```

### 2. 修改启动逻辑

**对齐 tbsrc 行为**:
```go
func (t *Trader) Start() error {
    // ... 启动各组件 ...

    // 根据运行模式决定是否自动激活
    if t.Config.System.Mode == "simulation" {
        // Simulation 模式：自动激活
        if err := t.Strategy.Start(); err != nil {
            return err
        }
        log.Println("[Trader] ✓ Strategy started (auto-activated in simulation mode)")
    } else {
        // Live 模式：等待手动激活
        log.Println("[Trader] Strategy initialized but NOT activated")
        log.Println("[Trader] Waiting for manual activation signal...")
        log.Println("[Trader] Use: kill -SIGUSR1 <PID> to activate")
    }

    // 启动信号处理器
    t.setupSignalHandlers()

    return nil
}
```

### 3. 添加控制脚本

**startTrade.sh**:
```bash
#!/bin/bash
STRATEGY_ID=$1

if [ -z "$STRATEGY_ID" ]; then
    echo "Usage: $0 <strategy_id>"
    exit 1
fi

# 读取 PID
PID=$(cat trader.$STRATEGY_ID.pid)

if [ -z "$PID" ]; then
    echo "Error: Strategy $STRATEGY_ID not found"
    exit 1
fi

# 发送激活信号
kill -SIGUSR1 $PID
echo "✓ Strategy $STRATEGY_ID activation signal sent"
```

**stopTrade.sh**:
```bash
#!/bin/bash
STRATEGY_ID=$1

if [ -z "$STRATEGY_ID" ]; then
    echo "Usage: $0 <strategy_id>"
    exit 1
fi

# 读取 PID
PID=$(cat trader.$STRATEGY_ID.pid)

if [ -z "$PID" ]; then
    echo "Error: Strategy $STRATEGY_ID not found"
    exit 1
fi

# 发送停止信号
kill -SIGUSR2 $PID
echo "✓ Strategy $STRATEGY_ID deactivation signal sent (squareoff)"
```

---

## 推荐方案

### 方案：Unix 信号 + HTTP API（混合）

**理由**:
1. ✅ Unix 信号：与 tbsrc 完全一致，支持批量控制
2. ✅ HTTP API：现代化接口，支持远程控制和监控
3. ✅ 两者互补：简单场景用信号，复杂场景用 API

**实现优先级**:
1. **P0（立即）**: 添加 Unix 信号支持，对齐 tbsrc
2. **P1（重要）**: 修改启动逻辑（Live 模式不自动激活）
3. **P2（增强）**: 添加 HTTP API
4. **P3（完善）**: 添加命令行工具

---

## 实现示例

### 完整的信号处理实现

```go
// pkg/trader/trader.go

import (
    "os"
    "os/signal"
    "syscall"
)

type Trader struct {
    // ... existing fields ...
    controlSignals chan os.Signal
}

func (t *Trader) Start() error {
    // ... existing code ...

    // 根据运行模式决定初始状态
    autoStart := false
    if t.Config.System.Mode == "simulation" {
        autoStart = true
        log.Println("[Trader] Simulation mode: auto-activating strategy")
    } else if t.Config.System.Mode == "live" {
        autoStart = false
        log.Println("[Trader] Live mode: strategy waiting for manual activation")
        log.Println("[Trader] Send SIGUSR1 to activate: kill -SIGUSR1", os.Getpid())
    }

    if autoStart {
        if err := t.Strategy.Start(); err != nil {
            return err
        }
        log.Println("[Trader] ✓ Strategy started")
    }

    // 启动信号处理
    t.setupSignalHandlers()

    // ... rest of code ...
}

func (t *Trader) setupSignalHandlers() {
    t.controlSignals = make(chan os.Signal, 1)

    // 监听控制信号（SIGUSR1, SIGUSR2）
    signal.Notify(t.controlSignals, syscall.SIGUSR1, syscall.SIGUSR2)

    go t.handleControlSignals()
}

func (t *Trader) handleControlSignals() {
    for t.IsRunning() {
        sig := <-t.controlSignals

        switch sig {
        case syscall.SIGUSR1:
            // 激活策略（对应 tbsrc SIGUSR1）
            log.Println("[Trader] ════════════════════════════════════════")
            log.Println("[Trader] Received SIGUSR1: Activating strategy")
            log.Println("[Trader] ════════════════════════════════════════")

            t.Strategy.GetBaseStrategy().ControlState.ExitRequested = false
            t.Strategy.GetBaseStrategy().ControlState.CancelPending = false
            t.Strategy.GetBaseStrategy().ControlState.FlattenMode = false
            t.Strategy.GetBaseStrategy().ControlState.Activate()

            if err := t.Strategy.Start(); err != nil {
                log.Printf("[Trader] Error starting strategy: %v", err)
            } else {
                log.Println("[Trader] ✓ Strategy activated and trading")
            }

        case syscall.SIGUSR2:
            // 停止策略并平仓（对应 tbsrc SIGTSTP）
            log.Println("[Trader] ════════════════════════════════════════")
            log.Println("[Trader] Received SIGUSR2: Deactivating strategy (squareoff)")
            log.Println("[Trader] ════════════════════════════════════════")

            t.Strategy.TriggerFlatten(strategy.FlattenReasonManual, false)
            t.Strategy.GetBaseStrategy().ControlState.Deactivate()

            log.Println("[Trader] ✓ Strategy deactivated, positions being closed")
        }
    }
}
```

---

## 总结

### 当前状态

| 功能 | tbsrc | QuantlinkTrader | 状态 |
|------|-------|-----------------|------|
| Live 模式默认不激活 | ✅ | ❌ | ⚠️ **需要改进** |
| 外部信号控制 | ✅ | ❌ | ⚠️ **需要实现** |
| 手动激活/停止 | ✅ | ❌ | ⚠️ **需要实现** |
| 批量控制 | ✅ | ❌ | ⚠️ **需要实现** |
| 控制脚本 | ✅ | ❌ | ⚠️ **需要实现** |
| 时段自动控制 | ❌ | ✅ | ✅ **额外功能** |

### 行动项

**必须实现（对齐 tbsrc）**:
1. ✅ 添加 Unix 信号处理（SIGUSR1/SIGUSR2）
2. ✅ Live 模式启动时不自动激活
3. ✅ 创建控制脚本（startTrade.sh/stopTrade.sh）

**建议实现（增强功能）**:
4. ➕ 添加 HTTP REST API
5. ➕ 添加命令行控制工具
6. ➕ 保留时段自动控制（作为可选功能）

---

**文档版本**: 1.0.0
**最后更新**: 2026-01-22
**状态**: ⚠️ **需要改进以对齐 tbsrc**
