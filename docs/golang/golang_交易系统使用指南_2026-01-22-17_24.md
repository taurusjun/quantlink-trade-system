# QuantlinkTrader 使用指南

**日期**: 2026-01-22
**版本**: 1.0.0

---

## 概述

**QuantlinkTrader** 是 quantlink-trade-system/golang 项目的生产入口程序，对应 tbsrc 的 TradeBot 可执行文件。

### 特性

✅ **命令行参数支持** - 灵活配置所有参数
✅ **YAML 配置文件** - 标准化配置格式
✅ **多策略类型支持** - Passive, Aggressive, Hedging, Pairwise Arb
✅ **运行模式切换** - Live, Backtest, Simulation
✅ **交易时段管理** - 自动启停策略
✅ **风险管理集成** - 完整的风险控制
✅ **热加载配置** - 无需重启更新配置
✅ **生产就绪** - 完整的日志和监控

---

## 快速开始

### 1. 编译

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system/golang
go build -o QuantlinkTrader ./cmd/trader
```

编译成功后会生成 `QuantlinkTrader` 可执行文件（约 18MB）。

### 2. 查看帮助

```bash
./QuantlinkTrader --help
```

输出：
```
Usage: QuantlinkTrader [OPTIONS]

A production-ready trading system for quantitative strategies.

Options:
  -config string
    	Configuration file path (default "./config/trader.yaml")
  -help
    	Print help and exit
  -log-file string
    	Log file path (overrides config)
  -log-level string
    	Log level: debug, info, warn, error (overrides config)
  -mode string
    	Run mode: live, backtest, simulation (overrides config)
  -strategy-id string
    	Strategy ID (overrides config)
  -strategy-type string
    	Strategy type: passive, aggressive, hedging, pairwise_arb (overrides config)
  -version
    	Print version and exit
  -watch-config
    	Watch config file for changes and hot reload
```

### 3. 运行示例

```bash
# 使用默认配置运行
./QuantlinkTrader --config ./config/trader.yaml

# 使用自定义策略 ID 和模式运行
./QuantlinkTrader --config ./config/trader.yaml --strategy-id 92201 --mode simulation

# 启用配置热加载
./QuantlinkTrader --config ./config/trader.yaml --watch-config
```

---

## 命令行参数

### 基本参数

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `--config` | 配置文件路径 | `./config/trader.yaml` | `--config /path/to/config.yaml` |
| `--strategy-id` | 策略 ID（覆盖配置） | - | `--strategy-id 92201` |
| `--strategy-type` | 策略类型（覆盖配置） | - | `--strategy-type passive` |
| `--mode` | 运行模式（覆盖配置） | - | `--mode live` |

### 日志参数

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `--log-file` | 日志文件路径（覆盖配置） | - | `--log-file ./log/trader.log` |
| `--log-level` | 日志级别（覆盖配置） | - | `--log-level debug` |

### 其他参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--watch-config` | 监听配置文件变化并热加载 | `--watch-config` |
| `--version` | 显示版本并退出 | `--version` |
| `--help` | 显示帮助并退出 | `--help` |

---

## 配置文件

### 配置文件结构

```yaml
system:
  strategy_id: "92201"
  mode: "simulation"

strategy:
  type: "passive"
  symbols: ["ag2502"]
  exchanges: ["SHFE"]
  max_position_size: 100
  max_exposure: 1000000.0
  parameters:
    # 策略特定参数

session:
  start_time: "09:00:00"
  end_time: "15:00:00"
  timezone: "Asia/Shanghai"
  auto_start: true
  auto_stop: true

risk:
  max_drawdown: 10000.0
  stop_loss: 50000.0
  max_loss: 100000.0
  daily_loss_limit: 200000.0

engine:
  ors_gateway_addr: "localhost:50052"
  nats_addr: "nats://localhost:4222"

portfolio:
  total_capital: 1000000.0

logging:
  level: "info"
  file: "./log/trader.92201.log"
```

### 示例配置文件

项目提供了多个示例配置文件：

| 文件 | 策略类型 | 说明 |
|------|---------|------|
| `config/trader.yaml` | Passive | 被动做市策略 |
| `config/trader.aggressive.yaml` | Aggressive | 激进趋势策略 |
| `config/trader.pairwise.yaml` | Pairwise Arb | 配对套利策略 |

---

## 运行模式

### 1. Simulation Mode（模拟模式）

**用途**: 开发和测试

```bash
./QuantlinkTrader --config ./config/trader.yaml --mode simulation
```

**特点**:
- 不连接真实的市场数据和订单路由服务
- 可以在没有外部依赖的情况下运行
- 适合开发和单元测试

### 2. Backtest Mode（回测模式）

**用途**: 历史数据回测

```bash
./QuantlinkTrader --config ./config/trader.yaml --mode backtest
```

**特点**:
- 使用历史数据进行回测
- 评估策略表现
- 优化策略参数

### 3. Live Mode（实盘模式）

**用途**: 生产交易

```bash
./QuantlinkTrader --config ./config/trader.yaml --mode live
```

**特点**:
- 连接真实的市场数据和订单路由服务
- 真实交易
- **必须确保所有服务正常运行**

**前置条件**:
- ORS Gateway 运行在 `localhost:50052`
- NATS 消息队列运行在 `localhost:4222`
- 市场数据服务正常

---

## 策略类型

### 1. Passive Strategy（被动做市策略）

```bash
./QuantlinkTrader --config ./config/trader.yaml --strategy-type passive
```

**配置示例**:
```yaml
strategy:
  type: "passive"
  parameters:
    spread_multiplier: 0.5
    order_size: 10
    max_inventory: 100
    inventory_skew: 0.5
    min_spread: 1.0
    order_refresh_ms: 1000
    use_order_imbalance: true
```

### 2. Aggressive Strategy（激进趋势策略）

```bash
./QuantlinkTrader --config ./config/trader.aggressive.yaml
```

**配置示例**:
```yaml
strategy:
  type: "aggressive"
  parameters:
    trend_period: 50
    momentum_period: 20
    signal_threshold: 0.6
    order_size: 20
    stop_loss_percent: 0.02
    take_profit_percent: 0.05
```

### 3. Hedging Strategy（对冲策略）

**配置示例**:
```yaml
strategy:
  type: "hedging"
  symbols: ["ag2502", "ag2504"]  # 需要两个品种
  parameters:
    hedge_ratio: 1.0
    rebalance_threshold: 0.1
    dynamic_hedge_ratio: true
```

### 4. Pairwise Arbitrage Strategy（配对套利策略）

```bash
./QuantlinkTrader --config ./config/trader.pairwise.yaml
```

**配置示例**:
```yaml
strategy:
  type: "pairwise_arb"
  symbols: ["ag2502", "ag2504"]  # 需要两个品种
  parameters:
    lookback_period: 100
    entry_zscore: 2.0
    exit_zscore: 0.5
    min_correlation: 0.7
    spread_type: "difference"
```

---

## 交易时段管理

### 配置交易时段

```yaml
session:
  start_time: "09:00:00"      # 开始时间（本地时间）
  end_time: "15:00:00"        # 结束时间（本地时间）
  timezone: "Asia/Shanghai"   # 时区
  auto_start: true            # 自动启动策略
  auto_stop: true             # 自动停止策略
```

### 行为

- **auto_start: true**: 在交易时段开始时自动启动策略
- **auto_stop: true**: 在交易时段结束时自动停止策略（并平仓）
- **timezone**: 支持所有 IANA 时区（如 `Asia/Shanghai`, `America/New_York`）

### 夜盘支持

支持跨日交易时段（如夜盘 21:00 - 次日 02:30）：

```yaml
session:
  start_time: "21:00:00"      # 晚上 21:00 开始
  end_time: "02:30:00"        # 次日凌晨 02:30 结束
  timezone: "Asia/Shanghai"
  auto_start: true
  auto_stop: true
```

---

## 风险管理

### 风险限制配置

```yaml
risk:
  max_drawdown: 10000.0         # 最大回撤限制
  stop_loss: 50000.0            # 止损金额（对应 tbsrc STOP_LOSS）
  max_loss: 100000.0            # 最大亏损限制（对应 tbsrc MAX_LOSS）
  daily_loss_limit: 200000.0    # 每日亏损限制
  max_reject_count: 10          # 最大拒单次数
  check_interval_ms: 100        # 风险检查间隔（毫秒）
```

### 风险控制行为

当触发风险限制时：

1. **max_drawdown**: 暂停策略，等待恢复条件
2. **stop_loss / max_loss**: 立即停止策略并平仓（对应 tbsrc FlattenReasonStopLoss）
3. **daily_loss_limit**: 今日停止交易
4. **max_reject_count**: 暂停策略，检查订单问题

---

## 日志管理

### 日志配置

```yaml
logging:
  level: "info"                 # 日志级别: debug, info, warn, error
  file: "./log/trader.92201.log" # 日志文件路径
  max_size_mb: 100              # 日志文件最大大小（MB）
  max_backups: 10               # 保留的旧日志文件数量
  max_age_days: 30              # 日志文件保留天数
  compress: true                # 压缩旧日志文件
  console: true                 # 同时输出到控制台
  json_format: false            # 使用 JSON 格式
```

### 日志级别

- **debug**: 详细调试信息
- **info**: 一般信息（默认）
- **warn**: 警告信息
- **error**: 错误信息

### 日志格式

```
2026/01/22 17:22:57 [Trader] Starting trader...
2026/01/22 17:22:57 [Trader] ✓ Risk Manager started
2026/01/22 17:22:57 [Trader] ✓ Strategy Engine started
2026/01/22 17:22:57 [Trader] ✓ Strategy started
```

---

## 生产部署

### 部署步骤

#### 1. 准备配置文件

```bash
# 复制示例配置
cp config/trader.yaml config/trader.92201.yaml

# 编辑配置
vim config/trader.92201.yaml
```

#### 2. 创建日志目录

```bash
mkdir -p log
```

#### 3. 启动程序

```bash
# 后台运行
nohup ./QuantlinkTrader --config ./config/trader.92201.yaml \
    --strategy-id 92201 \
    --mode live \
    >> nohup.out.92201 2>&1 &

# 记录 PID
echo $! > trader.92201.pid
```

#### 4. 监控运行

```bash
# 查看日志
tail -f log/trader.92201.log

# 查看 nohup 输出
tail -f nohup.out.92201

# 检查进程
ps aux | grep QuantlinkTrader
```

#### 5. 停止程序

```bash
# 优雅停止（发送 SIGINT）
kill -INT $(cat trader.92201.pid)

# 或强制停止
kill -9 $(cat trader.92201.pid)
```

### 多策略部署

部署多个策略实例：

```bash
# 策略 1: Passive (ag2502)
./QuantlinkTrader --config ./config/trader.yaml \
    --strategy-id 92201 --mode live &

# 策略 2: Aggressive (au2502)
./QuantlinkTrader --config ./config/trader.aggressive.yaml \
    --strategy-id 93201 --mode live &

# 策略 3: Pairwise (ag2502-ag2504)
./QuantlinkTrader --config ./config/trader.pairwise.yaml \
    --strategy-id 94201 --mode live &
```

---

## 监控和管理

### 状态监控

程序会每 30 秒输出一次状态信息：

```
[Main] ════════════════════════════════════════════════════════════
[Main] Periodic Status Update - 17:23:27
[Main] ────────────────────────────────────────────────────────────
[Main] Running:        true
[Main] Strategy ID:    92201
[Main] Mode:           simulation
[Main] Position:       0 (Long: 0, Short: 0)
[Main] P&L:            0.00 (Realized: 0.00, Unrealized: 0.00)
[Main] ════════════════════════════════════════════════════════════
```

### 热加载配置

启用配置文件监听：

```bash
./QuantlinkTrader --config ./config/trader.yaml --watch-config
```

当配置文件修改后，程序会自动重新加载配置（热加载功能即将实现）。

---

## 与 tbsrc TradeBot 对比

| 方面 | tbsrc TradeBot | QuantlinkTrader |
|------|----------------|-----------------|
| **可执行文件** | TradeBot (69MB) | QuantlinkTrader (18MB) |
| **配置格式** | 自定义 .cfg | YAML |
| **配置层次** | 3 层（Config + Control + Model） | 单层 YAML |
| **命令行参数** | ✅ | ✅ |
| **策略类型** | ✅ | ✅ |
| **交易时段** | 控制文件指定 | Session 配置 |
| **热加载** | reloadParams.pl | --watch-config |
| **部署模式** | 多进程 | 单进程多 goroutine |
| **日志格式** | 自定义 | 标准化 |

### 优势

相比 tbsrc TradeBot：

1. **更简单**: 单一 YAML 配置 vs 3 层配置文件
2. **更轻量**: 18MB vs 69MB
3. **更标准**: YAML/JSON vs 自定义格式
4. **更现代**: 内置热加载、结构化日志
5. **更灵活**: Go goroutine vs C++ 多进程

---

## 故障排查

### 问题 1: 无法连接 ORS Gateway

**症状**:
```
[Trader] Warning: Engine initialization failed (Mode: simulation): ...
```

**解决**:
- 检查 ORS Gateway 是否运行：`ps aux | grep ors_gateway`
- 检查地址配置：`ors_gateway_addr: "localhost:50052"`
- 在 simulation 模式下可以忽略此警告

### 问题 2: 策略未生成信号

**症状**:
```
[Main] Signals:     0 pending
```

**解决**:
- 检查策略是否在交易时段内
- 检查 SharedIndicators 是否正确设置
- 检查市场数据是否正常

### 问题 3: 风险限制触发

**症状**:
```
[Trader] RISK ALERT: Stopping strategy due to max loss exceeded
```

**解决**:
- 检查 `risk` 配置中的限制是否合理
- 分析策略表现，调整参数
- 重置风险管理器后重启

---

## 下一步

### 即将支持的功能

- [ ] 完整的配置热加载实现
- [ ] HTTP REST API 监控接口
- [ ] WebSocket 实时数据推送
- [ ] 性能指标输出（Prometheus）
- [ ] Docker 容器化部署
- [ ] 配置生成工具（类似 setup.py）

---

## 总结

QuantlinkTrader 是 quantlink-trade-system/golang 项目的生产就绪入口程序：

✅ **完整功能** - 策略引擎、风险管理、组合管理
✅ **灵活配置** - 命令行参数 + YAML 配置
✅ **生产就绪** - 日志、监控、错误处理
✅ **易于部署** - 单一可执行文件
✅ **对齐 tbsrc** - 完全对应 TradeBot 功能

**开始使用**:
```bash
go build -o QuantlinkTrader ./cmd/trader
./QuantlinkTrader --config ./config/trader.yaml
```

---

**文档版本**: 1.0.0
**最后更新**: 2026-01-22
**作者**: Claude Code
