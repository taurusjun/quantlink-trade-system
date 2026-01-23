# Trader Configuration Files Guide

## 📋 Configuration Files Overview

### 1. **Base Configurations** (基础配置)

| 配置文件 | 策略类型 | 用途 | Strategy ID | 交易时段 | 端口 |
|---------|---------|------|-------------|---------|------|
| `trader.yaml` | Passive | **生产环境** - 被动做市策略 | 92201 | 09:00-15:00 | 9201 |
| `trader.test.yaml` | Pairwise Arb | **测试环境** - 配对套利 | test_92201 | 全天(00:00-23:59) | 9201 |

### 2. **Strategy-Specific Configurations** (策略专用配置)

| 配置文件 | 策略类型 | 说明 |
|---------|---------|------|
| `trader.pairwise.yaml` | Pairwise Arb | 配对套利策略模板 |
| `trader.aggressive.yaml` | Aggressive | 激进交易策略模板 |

### 3. **Product-Specific Configurations** (品种专用配置)

| 配置文件 | 品种对 | 交易所 | 说明 |
|---------|--------|--------|------|
| `trader.ag2502.ag2504.yaml` | 白银 2502/2504 | SHFE | 白银跨期配对 |
| `trader.al2502.al2503.yaml` | 铝 2502/2503 | SHFE | 铝跨期配对 |
| `trader.rb2505.rb2510.yaml` | 螺纹钢 2505/2510 | SHFE | 螺纹钢跨期配对 |

---

## 🚀 Quick Start Guide

### 启动不同策略

```bash
# 进入项目目录
cd /Users/user/PWorks/RD/quantlink-trade-system/golang

# ═══════════════════════════════════════════════════════
# 方式 1: 使用配置文件启动
# ═══════════════════════════════════════════════════════

# 启动被动做市策略（生产环境）
./bin/trader -config config/trader.yaml

# 启动配对套利测试环境
./bin/trader -config config/trader.test.yaml

# 启动激进策略
./bin/trader -config config/trader.aggressive.yaml

# 启动特定品种的配对交易
./bin/trader -config config/trader.ag2502.ag2504.yaml

# ═══════════════════════════════════════════════════════
# 方式 2: 使用快捷脚本（推荐）
# ═══════════════════════════════════════════════════════

# 见下方 "创建快捷脚本" 部分
```

### 停止运行的Trader

```bash
# 停止所有trader进程
pkill -f "bin/trader"

# 或者优雅停止（发送SIGTERM）
pkill -TERM -f "bin/trader"
```

### 查看运行状态

```bash
# 查看trader进程
ps aux | grep "bin/trader" | grep -v grep

# 查看API端口占用
lsof -i :9201

# 查看日志
tail -f log/trader.test.log
tail -f log/trader.92201.log
```

---

## 🔧 Configuration File Structure

### 关键配置项说明

```yaml
system:
  strategy_id: "92201"          # 策略唯一标识符（必须唯一）
  mode: "simulation"            # 运行模式: live, backtest, simulation

strategy:
  type: "passive"               # 策略类型: passive, aggressive, hedging, pairwise_arb
  symbols:                      # 交易品种
    - "ag2502"                  # Passive/Aggressive: 单个品种
    - "ag2504"                  # Pairwise: 需要两个品种
  exchanges:                    # 交易所
    - "SHFE"

session:
  start_time: "09:00:00"        # 交易开始时间（测试模式用00:00:00）
  end_time: "15:00:00"          # 交易结束时间（测试模式用23:59:59）
  auto_stop: true               # 自动停止（测试模式设为false）

api:
  enabled: true                 # 启用API
  port: 9201                    # API端口（多实例需要不同端口）
  host: "localhost"

logging:
  file: "./log/trader.92201.log"  # 日志文件（每个策略独立）
  level: "info"                   # 日志级别: debug, info, warn, error
```

---

## 📦 Creating New Configurations

### 创建新的策略配置

```bash
# 1. 复制现有配置作为模板
cp config/trader.test.yaml config/trader.my_strategy.yaml

# 2. 编辑新配置文件
vim config/trader.my_strategy.yaml

# 3. 修改关键字段：
#    - system.strategy_id: "my_strategy_001"
#    - api.port: 9202  (如果要并发运行多个策略)
#    - logging.file: "./log/trader.my_strategy.log"
#    - portfolio.strategy_allocation: "my_strategy_001": 0.3

# 4. 启动测试
./bin/trader -config config/trader.my_strategy.yaml
```

---

## 🎯 Strategy Types Explained

### 1. Passive Strategy (被动做市策略)
- **用途**: 提供流动性，赚取买卖价差
- **参数**: spread_multiplier, order_size, max_inventory
- **配置文件**: `trader.yaml`

### 2. Pairwise Arbitrage (配对套利策略)
- **用途**: 交易相关品种的价差，统计套利
- **参数**: spread_type, entry_zscore, exit_zscore, lookback_period
- **配置文件**: `trader.test.yaml`, `trader.pairwise.yaml`
- **要求**: 必须配置两个symbols

### 3. Aggressive Strategy (激进策略)
- **用途**: 主动寻找交易机会，追逐价格
- **参数**: aggression_level, min_edge, max_chase_levels
- **配置文件**: `trader.aggressive.yaml`

### 4. Hedging Strategy (对冲策略)
- **用途**: 风险对冲，保护现有持仓
- **参数**: hedge_ratio, rehedge_threshold
- **配置文件**: (待创建)

---

## 🔄 Multi-Strategy Setup

### 同时运行多个策略

```bash
# 策略1: 白银配对套利
./bin/trader -config config/trader.ag2502.ag2504.yaml &

# 策略2: 铝配对套利（需要修改端口）
# 编辑config/trader.al2502.al2503.yaml, 将 api.port 改为 9202
./bin/trader -config config/trader.al2502.al2503.yaml &

# 策略3: 螺纹钢配对套利（需要修改端口）
# 编辑config/trader.rb2505.rb2510.yaml, 将 api.port 改为 9203
./bin/trader -config config/trader.rb2505.rb2510.yaml &

# 查看所有运行的策略
ps aux | grep "bin/trader" | grep -v grep
```

**注意事项**:
- 每个策略必须有唯一的 `strategy_id`
- 如果要同时运行，每个策略需要不同的 `api.port`
- 日志文件 `logging.file` 必须不同
- Portfolio分配总和不要超过100%

---

## 🛠️ Troubleshooting

### 常见问题

#### 1. API连接失败
```bash
# 检查trader是否运行
ps aux | grep trader

# 检查端口是否监听
lsof -i :9201

# 查看日志
tail -f log/trader.test.log
```

#### 2. 策略不交易
- 检查 `session.start_time` 和 `end_time` 是否在当前时间范围内
- 测试时建议使用 `trader.test.yaml` (全天运行)
- 检查 `session.auto_stop` 是否为 false

#### 3. 两条腿不显示
- 确认使用的是 `pairwise_arb` 策略类型
- 确认 `symbols` 配置了两个品种
- 检查市场数据是否正常接收

#### 4. 端口冲突
```bash
# 查找占用端口的进程
lsof -i :9201

# 杀死进程
kill -9 <PID>

# 或者修改配置文件使用不同端口
```

---

## 📊 Web UI Connection

### 连接到不同策略的Web UI

```
# 默认策略
http://localhost:3000/?api=http://localhost:9201

# 多策略情况
策略1: http://localhost:3000/?api=http://localhost:9201
策略2: http://localhost:3000/?api=http://localhost:9202
策略3: http://localhost:3000/?api=http://localhost:9203
```

---

## 📝 Configuration Best Practices

1. **测试先行**: 新策略先用 `simulation` 模式测试
2. **独立日志**: 每个策略使用独立的日志文件
3. **唯一标识**: strategy_id 必须全局唯一
4. **端口规划**: 提前规划好每个策略的API端口
5. **资金分配**: portfolio.strategy_allocation 总和控制在1.0以内
6. **风险控制**: 根据策略特点调整 risk 参数
7. **版本管理**: 配置文件加入git，跟踪变更

---

## 🔗 Related Files

- `continuous_market_sim.sh` - 市场数据模拟脚本
- `start_trader.sh` - 启动脚本（待创建）
- `log/trader.*.log` - 策略日志文件
- `web-ui/index.html` - Web UI界面
