# 策略执行入口分析：Golang vs tbsrc

**日期**: 2026-01-22
**目的**: 分析 quantlink-trade-system/golang 项目的策略执行入口，并与 tbsrc 对比

---

## 问题：Golang 项目缺少生产入口

### 现状

**quantlink-trade-system/golang** 项目目前只有 **demo 程序**，缺少类似 tbsrc TradeBot 的完整生产可执行文件入口。

### 现有 Demo 程序

项目中现有的可执行程序都在 `cmd/` 目录下：

| Demo 程序 | 文件路径 | 功能 |
|-----------|---------|------|
| **strategy_demo** | `cmd/strategy_demo/main.go` | 单策略演示（PassiveStrategy + StrategyEngine） |
| **all_strategies_demo** | `cmd/all_strategies_demo/main.go` | 4 种策略演示（Passive, Aggressive, Hedging, Pairs） |
| **integrated_demo** | `cmd/integrated_demo/main.go` | 完整系统演示（Engine + Portfolio + Risk） |
| **indicator_demo** | `cmd/indicator_demo/main.go` | 指标库演示 |
| **md_client** | `cmd/md_client/main.go` | 市场数据客户端 |
| **ors_client** | `cmd/ors_client/main.go` | 订单路由客户端 |

**特点**:
- ✅ 功能完整（包含 Engine、Portfolio、Risk 等组件）
- ✅ 可以运行和测试
- ❌ **不支持命令行参数配置**（硬编码配置）
- ❌ **不支持从配置文件加载**
- ❌ **不支持多策略独立部署**
- ❌ **不是生产就绪的可执行文件**

---

## tbsrc TradeBot 的入口分析

### 入口特点

**可执行文件**: `TradeBot` (69MB C++ 编译的二进制文件)

**启动方式**:
```bash
./TradeBot --Live \
    --controlFile ./controls/day/control.ag2502.ag2504.par.txt.92201 \
    --strategyID 92201 \
    --configFile ./config/config_CHINA.92201.cfg \
    --adjustLTP 1 \
    --printMod 1 \
    --updateInterval 300000 \
    --logFile ./log/log.control.ag2502.ag2504.par.txt.92201.20241226 \
    >> nohup.out.92201 2>&1 &
```

### 关键参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--Live` | 实盘模式标志 | `--Live` |
| `--controlFile` | 控制文件路径 | `./controls/day/control.ag2502.ag2504.par.txt.92201` |
| `--strategyID` | 策略唯一标识符 | `92201` |
| `--configFile` | 配置文件路径 | `./config/config_CHINA.92201.cfg` |
| `--adjustLTP` | LTP 调整标志 | `1` |
| `--printMod` | 打印模式 | `1` |
| `--updateInterval` | 更新间隔（毫秒） | `300000` |
| `--logFile` | 日志文件路径 | `./log/log.control.ag2502.ag2504.par.txt.92201.20241226` |

### 配置层次

```
TradeBot 可执行文件
    ↓ (命令行参数)
Config File (config_CHINA.92201.cfg)
    ├─ 共享内存键 (SHM_MD_KEY, SHM_ORS_KEY)
    ├─ 交易所配置 (EXCHANGE_NAME, EXCHANGE_ID)
    ├─ 线程配置 (CPU 亲和性, 调度策略)
    └─ 系统参数 (TICK_SIZE, CONTRACT_MULTIPLIER)
    ↓
Control File (control.ag2502.ag2504.par.txt.92201)
    ├─ 交易品种 (ag_F_2_SFE, ag_F_4_SFE)
    ├─ 模型文件路径
    ├─ 交易所 (SFE)
    ├─ 最大持仓 (16)
    ├─ 策略类型 (TB_PAIR_STRAT)
    └─ 交易时段 (0100 0700)
    ↓
Model File (model.ag2502.ag2504.par.txt.92201)
    ├─ 持仓管理参数 (SIZE, MAX_SIZE, MAX_QUOTE_LEVEL)
    ├─ 入场阈值 (BEGIN_PLACE, LONG_PLACE, SHORT_PLACE)
    └─ 风险控制参数 (STOP_LOSS, MAX_LOSS, UPNL_LOSS)
```

### tbsrc 入口代码结构

**main 函数** (推测结构):
```cpp
// tbsrc/main.cpp (推测)
int main(int argc, char** argv) {
    // 1. 解析命令行参数
    CommandLineArgs args = parseArgs(argc, argv);

    // 2. 加载配置文件 (config_CHINA.92201.cfg)
    Config config = loadConfig(args.configFile);

    // 3. 加载控制文件 (control.ag2502.ag2504.par.txt.92201)
    ControlFile controlFile = loadControlFile(args.controlFile);

    // 4. 加载模型文件 (model.ag2502.ag2504.par.txt.92201)
    ModelParams modelParams = loadModelFile(controlFile.modelFile);

    // 5. 初始化系统组件
    SharedMemoryManager shmMgr(config);
    MarketDataConnector mdConnector(config);
    OrderRoutingConnector orsConnector(config);

    // 6. 创建策略引擎
    StrategyEngine engine(config, controlFile, modelParams);

    // 7. 创建策略实例
    Strategy* strategy = createStrategy(
        controlFile.strategyType,  // TB_PAIR_STRAT
        args.strategyID,            // 92201
        controlFile,
        modelParams
    );

    // 8. 添加策略到引擎
    engine.addStrategy(strategy);

    // 9. 启动引擎
    engine.start();

    // 10. 主循环
    while (running) {
        // 处理市场数据
        // 处理订单回报
        // 定时检查
    }

    // 11. 清理和退出
    engine.stop();
    return 0;
}
```

---

## Golang 项目需要的生产入口

### 目标

创建一个类似 tbsrc TradeBot 的生产可执行文件：`QuantlinkTrader`

### 建议的入口程序

**文件路径**: `cmd/trader/main.go`

**功能需求**:
1. ✅ 支持命令行参数配置
2. ✅ 支持从 YAML/JSON 配置文件加载
3. ✅ 支持多策略类型（Passive, Aggressive, Hedging, Pairs）
4. ✅ 支持运行模式切换（Live, Backtest, Simulation）
5. ✅ 完整的系统集成（Engine + Portfolio + Risk）
6. ✅ 日志管理
7. ✅ 优雅退出和错误处理
8. ✅ 支持热加载配置

### 命令行参数设计

```bash
./QuantlinkTrader \
    --mode live \
    --config ./config/trader.yaml \
    --strategy-id 92201 \
    --strategy-type passive \
    --log-file ./log/trader.92201.20260122.log \
    --log-level info
```

| 参数 | 说明 | 默认值 | 必需 |
|------|------|--------|------|
| `--mode` | 运行模式 (live/backtest/sim) | `sim` | 否 |
| `--config` | 配置文件路径 | `./config/trader.yaml` | 是 |
| `--strategy-id` | 策略 ID | - | 是 |
| `--strategy-type` | 策略类型 | - | 否（可从配置文件读取） |
| `--log-file` | 日志文件路径 | `./log/trader.<strategyID>.<date>.log` | 否 |
| `--log-level` | 日志级别 | `info` | 否 |
| `--watch-config` | 监听配置文件变化 | `false` | 否 |

### 配置文件设计

**trader.yaml** 示例:
```yaml
# System Configuration
system:
  strategy_id: 92201
  mode: live  # live, backtest, simulation

# Strategy Configuration
strategy:
  type: passive  # passive, aggressive, hedging, pairwise_arb
  symbols:
    - ag2502
    - ag2504
  exchanges:
    - SHFE
  max_position_size: 100
  max_exposure: 1000000.0

  # Strategy-specific parameters
  parameters:
    # Passive Strategy
    spread_multiplier: 0.5
    order_size: 10
    max_inventory: 100
    inventory_skew: 0.5
    min_spread: 1.0
    order_refresh_ms: 1000
    use_order_imbalance: true

# Trading Session
session:
  start_time: "09:00:00"
  end_time: "15:00:00"
  timezone: "Asia/Shanghai"
  auto_start: true
  auto_stop: true

# Risk Limits
risk:
  max_drawdown: 10000.0
  stop_loss: 50000.0
  max_loss: 100000.0
  daily_loss_limit: 200000.0

# Engine Configuration
engine:
  ors_gateway_addr: "localhost:50052"
  nats_addr: "nats://localhost:4222"
  order_queue_size: 100
  timer_interval: 5s
  max_concurrent_orders: 10

# Logging
logging:
  level: info  # debug, info, warn, error
  file: "./log/trader.92201.20260122.log"
  max_size_mb: 100
  max_backups: 10
  max_age_days: 30
  compress: true
```

### 实现结构

**cmd/trader/main.go**:
```go
package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/quantlink-trade-system/pkg/config"
    "github.com/yourusername/quantlink-trade-system/pkg/trader"
)

func main() {
    // 1. Parse command line arguments
    var (
        configFile  = flag.String("config", "./config/trader.yaml", "Config file path")
        strategyID  = flag.String("strategy-id", "", "Strategy ID")
        mode        = flag.String("mode", "sim", "Run mode: live, backtest, sim")
        logFile     = flag.String("log-file", "", "Log file path")
        logLevel    = flag.String("log-level", "info", "Log level")
        watchConfig = flag.Bool("watch-config", false, "Watch config file for changes")
    )
    flag.Parse()

    fmt.Println("╔═══════════════════════════════════════════════════════════╗")
    fmt.Println("║            QuantLink Trader - Production                 ║")
    fmt.Println("╚═══════════════════════════════════════════════════════════╝")

    // 2. Load configuration
    cfg, err := config.LoadTraderConfig(*configFile)
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Override config with command line args
    if *strategyID != "" {
        cfg.System.StrategyID = *strategyID
    }
    if *mode != "" {
        cfg.System.Mode = *mode
    }

    // 3. Setup logging
    if *logFile != "" {
        cfg.Logging.File = *logFile
    }
    logger := setupLogging(cfg.Logging)
    defer logger.Close()

    // 4. Create trader instance
    trader, err := trader.NewTrader(cfg, logger)
    if err != nil {
        log.Fatalf("Failed to create trader: %v", err)
    }

    // 5. Initialize trader
    if err := trader.Initialize(); err != nil {
        log.Fatalf("Failed to initialize trader: %v", err)
    }

    // 6. Start config watcher (if enabled)
    if *watchConfig {
        go watchConfigFile(*configFile, trader)
    }

    // 7. Start trader
    if err := trader.Start(); err != nil {
        log.Fatalf("Failed to start trader: %v", err)
    }

    logger.Info("Trader started successfully")
    logger.Info("Strategy ID: %s, Mode: %s", cfg.System.StrategyID, cfg.System.Mode)

    // 8. Wait for interrupt
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // 9. Shutdown
    logger.Info("Shutting down trader...")
    if err := trader.Stop(); err != nil {
        logger.Error("Error during shutdown: %v", err)
    }

    logger.Info("Trader stopped successfully")
}
```

**pkg/trader/trader.go**:
```go
package trader

import (
    "fmt"

    "github.com/yourusername/quantlink-trade-system/pkg/config"
    "github.com/yourusername/quantlink-trade-system/pkg/strategy"
    "github.com/yourusername/quantlink-trade-system/pkg/portfolio"
    "github.com/yourusername/quantlink-trade-system/pkg/risk"
)

// Trader encapsulates the complete trading system
type Trader struct {
    Config      *config.TraderConfig
    Logger      Logger

    Engine      *strategy.StrategyEngine
    Strategy    strategy.Strategy
    Portfolio   *portfolio.PortfolioManager
    RiskManager *risk.RiskManager

    // Session management
    SessionMgr  *SessionManager
}

func NewTrader(cfg *config.TraderConfig, logger Logger) (*Trader, error) {
    t := &Trader{
        Config: cfg,
        Logger: logger,
    }

    // Create components
    if err := t.createComponents(); err != nil {
        return nil, err
    }

    return t, nil
}

func (t *Trader) Initialize() error {
    // Initialize all components
    // 1. Risk Manager
    // 2. Portfolio Manager
    // 3. Strategy Engine
    // 4. Strategy Instance
    // 5. Session Manager
    return nil
}

func (t *Trader) Start() error {
    // Start all components in order
    // 1. Risk Manager
    // 2. Portfolio Manager
    // 3. Strategy Engine
    // 4. Strategy Instance
    // 5. Session Manager (if auto_start)
    return nil
}

func (t *Trader) Stop() error {
    // Stop all components in reverse order
    return nil
}

func (t *Trader) ReloadConfig(newCfg *config.TraderConfig) error {
    // Hot reload configuration
    return nil
}

func (t *Trader) createComponents() error {
    // Create Risk Manager
    t.RiskManager = risk.NewRiskManager(&risk.RiskManagerConfig{
        EnableGlobalLimits:    true,
        EnableStrategyLimits:  true,
        EnablePortfolioLimits: true,
    })

    // Create Portfolio Manager
    t.Portfolio = portfolio.NewPortfolioManager(&portfolio.PortfolioConfig{
        TotalCapital: t.Config.Portfolio.TotalCapital,
    })

    // Create Strategy Engine
    t.Engine = strategy.NewStrategyEngine(&strategy.EngineConfig{
        ORSGatewayAddr: t.Config.Engine.ORSGatewayAddr,
        NATSAddr:       t.Config.Engine.NATSAddr,
    })

    // Create Strategy Instance (based on type)
    var err error
    t.Strategy, err = t.createStrategy()
    if err != nil {
        return fmt.Errorf("failed to create strategy: %w", err)
    }

    // Create Session Manager
    t.SessionMgr = NewSessionManager(t.Config.Session)

    return nil
}

func (t *Trader) createStrategy() (strategy.Strategy, error) {
    cfg := t.Config.Strategy

    var s strategy.Strategy

    switch cfg.Type {
    case "passive":
        s = strategy.NewPassiveStrategy(t.Config.System.StrategyID)
    case "aggressive":
        s = strategy.NewAggressiveStrategy(t.Config.System.StrategyID)
    case "hedging":
        s = strategy.NewHedgingStrategy(t.Config.System.StrategyID)
    case "pairwise_arb":
        s = strategy.NewPairwiseArbStrategy(t.Config.System.StrategyID)
    default:
        return nil, fmt.Errorf("unknown strategy type: %s", cfg.Type)
    }

    // Initialize strategy
    strategyConfig := &strategy.StrategyConfig{
        StrategyID:      t.Config.System.StrategyID,
        StrategyType:    cfg.Type,
        Symbols:         cfg.Symbols,
        Exchanges:       cfg.Exchanges,
        MaxPositionSize: cfg.MaxPositionSize,
        MaxExposure:     cfg.MaxExposure,
        RiskLimits:      cfg.RiskLimits,
        Parameters:      cfg.Parameters,
        Enabled:         true,
    }

    if err := s.Initialize(strategyConfig); err != nil {
        return nil, fmt.Errorf("failed to initialize strategy: %w", err)
    }

    return s, nil
}
```

---

## 实现计划

### Phase 1: 基础入口程序

**目标**: 创建最小可用的生产入口

**任务**:
1. ✅ 创建 `cmd/trader/main.go`
2. ✅ 实现命令行参数解析
3. ✅ 实现配置文件加载（YAML）
4. ✅ 创建 `pkg/trader/trader.go` 封装所有组件
5. ✅ 支持单策略运行
6. ✅ 基本日志功能

**示例命令**:
```bash
go build -o QuantlinkTrader ./cmd/trader
./QuantlinkTrader --config ./config/trader.yaml --strategy-id 92201
```

### Phase 2: 增强功能

**目标**: 添加生产必需功能

**任务**:
1. ✅ 实现配置文件热加载
2. ✅ 实现交易时段管理（SessionManager）
3. ✅ 增强日志（结构化日志、日志轮转）
4. ✅ 添加运行模式切换（Live/Backtest/Simulation）
5. ✅ 添加性能监控和指标输出
6. ✅ 添加健康检查端点

### Phase 3: 部署工具

**目标**: 简化生产部署

**任务**:
1. ✅ 创建配置生成工具（类似 setup.py）
2. ✅ 创建启动脚本生成器
3. ✅ 创建监控脚本（类似 pnl_watch.sh）
4. ✅ Docker 容器化
5. ✅ 部署文档

---

## 对比总结

| 方面 | tbsrc TradeBot | golang QuantlinkTrader (建议) |
|------|----------------|-------------------------------|
| **入口程序** | `TradeBot` (C++ 二进制) | `QuantlinkTrader` (Go 二进制) |
| **命令行参数** | ✅ 完整支持 | ✅ 需要实现 |
| **配置文件** | ✅ 自定义格式 (.cfg) | ✅ YAML/JSON（更标准） |
| **多层配置** | ✅ Config + Control + Model | ✅ 单一 YAML（更简单） |
| **策略类型** | ✅ 通过 strategyType 指定 | ✅ 通过 strategy.type 指定 |
| **部署模式** | ✅ 多进程（每策略独立） | ✅ 单进程多 goroutine |
| **热加载** | ✅ reloadParams.pl | ✅ 配置文件监听 |
| **日志管理** | ✅ 自定义格式 | ✅ 结构化日志（JSON） |
| **监控** | ✅ 外部脚本 (pnl_watch) | ✅ 内置 + REST API |
| **交易时段** | ✅ 控制文件指定 | ✅ SessionManager |

---

## 结论

### 当前状态

quantlink-trade-system/golang 项目 **缺少生产入口程序**，只有 demo。

### 建议

1. **立即实现**: 创建 `cmd/trader/main.go` 作为生产入口
2. **优先级**:
   - 🔴 **P0**: 命令行参数 + 配置文件加载
   - 🟠 **P1**: Trader 封装 + 策略类型支持
   - 🟡 **P2**: 热加载 + 交易时段管理
   - 🟢 **P3**: 部署工具 + 监控脚本

3. **优势**: Go 实现比 tbsrc 更简单
   - 单一 YAML 配置（vs 3 层配置文件）
   - 内置热加载（vs 外部脚本）
   - 统一日志格式（vs 自定义格式）
   - goroutine 模型（vs 多进程）

---

**下一步**: 实现 `cmd/trader/main.go` 和 `pkg/trader/trader.go`

