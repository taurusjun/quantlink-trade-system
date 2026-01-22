# 部署方式对比：QuantlinkTrader vs tbsrc TradeBot

**日期**: 2026-01-22

---

## 核心概念对比

### tbsrc TradeBot

**启动粒度**: 每个**交易对/策略实例**一个进程

```bash
# 策略实例 92201: ag2502-ag2504 配对交易
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

**配置文件**:
- `config_CHINA.92201.cfg` - 引擎配置（共享内存键、线程配置等）
- `control.ag2502.ag2504.par.txt.92201` - 控制文件（交易对、策略类型、时段）
- `model.ag2502.ag2504.par.txt.92201` - 模型参数（入场阈值、风险限制）

### QuantlinkTrader

**启动粒度**: 每个**交易对/策略实例**一个进程（相同！）

```bash
# 策略实例 92201: ag2502-ag2504 配对交易
./QuantlinkTrader \
    --config ./config/trader.ag2502.ag2504.yaml \
    --strategy-id 92201 \
    --mode live \
    --log-file ./log/trader.ag2502.ag2504.92201.log \
    >> nohup.out.92201 2>&1 &
```

**配置文件**:
- `trader.ag2502.ag2504.yaml` - 单一 YAML 配置（包含所有配置）

---

## 详细对比

### 1. 配置文件对应关系

| tbsrc | QuantlinkTrader | 说明 |
|-------|-----------------|------|
| `config_CHINA.92201.cfg` | `engine` section | 引擎配置 |
| `control.ag2502.ag2504.par.txt.92201` | `strategy` + `session` sections | 策略和时段配置 |
| `model.ag2502.ag2504.par.txt.92201` | `strategy.parameters` + `risk` sections | 参数和风险配置 |

#### tbsrc 配置示例

**config_CHINA.92201.cfg**:
```ini
SHM_MD_KEY = 5592201
SHM_ORS_KEY = 5692201
EXCHANGE_NAME = SFE
STRATEGY_THREAD_CPU_AFFINITY = 20
```

**control.ag2502.ag2504.par.txt.92201**:
```
ag_F_2_SFE ./models/model.ag2502.ag2504.par.txt.92201 SFE 16 TB_PAIR_STRAT 0100 0700 ag_F_4_SFE
```

**model.ag2502.ag2504.par.txt.92201**:
```
ag_F_2_SFE FUTCOM Dependant 0 MID_PX
ag_F_4_SFE FUTCOM Dependant 0 MID_PX
SIZE 4
MAX_SIZE 16
BEGIN_PLACE 5.006894
LONG_PLACE 7.510341
SHORT_PLACE 2.503447
STOP_LOSS 100000
MAX_LOSS 100000
```

#### QuantlinkTrader 配置示例

**trader.ag2502.ag2504.yaml** (统一在一个文件中):
```yaml
system:
  strategy_id: "92201"
  mode: "live"

strategy:
  type: "pairwise_arb"               # 对应 TB_PAIR_STRAT
  symbols: ["ag2502", "ag2504"]      # 对应 ag_F_2_SFE, ag_F_4_SFE
  exchanges: ["SHFE", "SHFE"]        # 对应 SFE
  max_position_size: 16              # 对应 controlFile 中的 16

  parameters:
    order_size: 4                    # 对应 SIZE
    entry_zscore: 2.0                # 对应 BEGIN_PLACE
    # ... 其他参数

session:
  start_time: "09:00:00"             # 对应 0100 (UTC+8)
  end_time: "15:00:00"               # 对应 0700 (UTC+8)
  timezone: "Asia/Shanghai"

risk:
  stop_loss: 100000.0                # 对应 STOP_LOSS
  max_loss: 100000.0                 # 对应 MAX_LOSS

engine:
  ors_gateway_addr: "localhost:50052"
  nats_addr: "nats://localhost:4222"

logging:
  file: "./log/trader.ag2502.ag2504.92201.log"
```

**优势**: ✅ 单一配置文件，更易管理

---

### 2. 启动命令对比

#### 场景 1: 单个策略实例

**tbsrc**:
```bash
nohup ./TradeBot --Live \
    --controlFile ./controls/day/control.ag2502.ag2504.par.txt.92201 \
    --strategyID 92201 \
    --configFile ./config/config_CHINA.92201.cfg \
    --adjustLTP 1 \
    --printMod 1 \
    --updateInterval 300000 \
    --logFile ./log/log.control.ag2502.ag2504.par.txt.92201.20241226 \
    >> nohup.out.92201 2>&1 &
```

**QuantlinkTrader**:
```bash
nohup ./QuantlinkTrader \
    --config ./config/trader.ag2502.ag2504.yaml \
    --strategy-id 92201 \
    --mode live \
    >> nohup.out.92201 2>&1 &
```

**对比**:
- ✅ **更简洁**: 6 个参数 vs 8 个参数
- ✅ **更清晰**: 配置文件包含大部分配置，命令行只需指定关键参数

#### 场景 2: 多个策略实例

**tbsrc** (TradeBot_China/bin/start.comms.night.sh):
```bash
#!/bin/bash
nohup ./TradeBot --Live --controlFile ./controls/night/control.ag2502.ag2504.par.txt.92201 \
    --strategyID 92201 --configFile ./config/config_CHINA.control.ag2502.ag2504.par.txt.92201.cfg \
    --adjustLTP 1 --printMod 1 --updateInterval 2000 \
    --logFile ./log/log.control.ag2502.ag2504.par.txt.92201.20241226.night \
    >> nohup.out.control.ag2502.ag2504.par.txt.92201 2>&1 &

nohup ./TradeBot --Live --controlFile ./controls/night/control.al2502.al2503.par.txt.93201 \
    --strategyID 93201 --configFile ./config/config_CHINA.control.al2502.al2503.par.txt.93201.cfg \
    --adjustLTP 1 --printMod 1 --updateInterval 2000 \
    --logFile ./log/log.control.al2502.al2503.par.txt.93201.20241226.night \
    >> nohup.out.control.al2502.al2503.par.txt.93201 2>&1 &
```

**QuantlinkTrader** (start_all_strategies.sh):
```bash
#!/bin/bash
nohup ./QuantlinkTrader \
    --config ./config/trader.ag2502.ag2504.yaml \
    --strategy-id 92201 --mode live \
    >> nohup.out.92201 2>&1 &

nohup ./QuantlinkTrader \
    --config ./config/trader.al2502.al2503.yaml \
    --strategy-id 93201 --mode live \
    >> nohup.out.93201 2>&1 &

nohup ./QuantlinkTrader \
    --config ./config/trader.rb2505.rb2510.yaml \
    --strategy-id 41231 --mode live \
    >> nohup.out.41231 2>&1 &
```

**对比**:
- ✅ **更简洁**: 命令行更短
- ✅ **更易读**: 配置文件名称清晰表达交易对

---

### 3. 部署流程对比

#### tbsrc 部署流程

```bash
# 1. 准备控制文件列表
cat controls_list
control.ag2502.ag2504.par.txt.92201
control.al2502.al2503.par.txt.93201
control.rb2505.rb2510.par.txt.41231

# 2. 运行 setup.py 生成配置和启动脚本
python setup.py
# 生成:
# - config/config_CHINA.control.*.cfg (每个策略一个)
# - controls/night/* (夜盘控制文件)
# - controls/day/* (日盘控制文件)
# - start.comms.night.sh
# - start.comms.am.sh
# - start.comms.pm.sh

# 3. 启动所有策略
./start.comms.night.sh
```

#### QuantlinkTrader 部署流程

```bash
# 1. 准备配置文件（手动或工具生成）
ls config/
trader.ag2502.ag2504.yaml
trader.al2502.al2503.yaml
trader.rb2505.rb2510.yaml

# 2. 启动所有策略（更简单！）
./start_all_strategies.sh

# 或手动启动单个策略
./QuantlinkTrader --config ./config/trader.ag2502.ag2504.yaml \
    --strategy-id 92201 --mode live &
```

**对比**:
- ✅ **更简单**: 不需要复杂的 setup.py
- ✅ **更直接**: 配置文件即所见即所得
- ⚠️ **需要工具**: 如果有很多策略，建议开发配置生成工具

---

### 4. 进程管理对比

#### tbsrc

**查看进程**:
```bash
ps aux | grep TradeBot
# 输出:
# user 12345 ... ./TradeBot ... 92201 ...
# user 12346 ... ./TradeBot ... 93201 ...
# user 12347 ... ./TradeBot ... 41231 ...
```

**停止策略**:
```bash
# 使用 tbstop 命令（需要找到 PID）
tbstop 92201

# 或直接 kill
kill <PID>
```

#### QuantlinkTrader

**查看进程**:
```bash
ps aux | grep QuantlinkTrader
# 输出:
# user 12345 ... ./QuantlinkTrader ... --strategy-id 92201 ...
# user 12346 ... ./QuantlinkTrader ... --strategy-id 93201 ...
# user 12347 ... ./QuantlinkTrader ... --strategy-id 41231 ...
```

**停止策略**:
```bash
# 使用 PID 文件
kill -INT $(cat trader.92201.pid)

# 或停止所有策略
./stop_all_strategies.sh
```

**对比**:
- ✅ **PID 管理**: 自动保存 PID 到文件
- ✅ **批量停止**: 提供停止脚本
- ✅ **优雅退出**: 使用 SIGINT 信号

---

### 5. 日志管理对比

#### tbsrc

**日志文件命名**:
```
log/log.control.ag2502.ag2504.par.txt.92201.20241226.night
```

**特点**:
- ❌ 文件名很长
- ❌ 需要在启动时指定日期
- ✅ 明确包含时段（night/am/pm）

#### QuantlinkTrader

**日志文件命名**:
```
log/trader.ag2502.ag2504.92201.log
```

**特点**:
- ✅ 文件名简洁
- ✅ 自动日志轮转（不需要日期）
- ✅ 压缩旧日志
- ✅ 配置中指定保留策略

**配置**:
```yaml
logging:
  file: "./log/trader.ag2502.ag2504.92201.log"
  max_size_mb: 100        # 100MB 后轮转
  max_backups: 10         # 保留 10 个备份
  max_age_days: 30        # 保留 30 天
  compress: true          # 压缩旧日志
```

---

### 6. 监控对比

#### tbsrc

**PNL 监控** (pnl_watch.sh):
```bash
#!/bin/bash
filepath='/home/TradeBot/TradeBot_Multi/main/log.live.control.*'
for i in $filepath$symbol*$currentDate; do
    grep "Trade:" $i | tail -1 | awk '{print $15}'
done
```

**特点**:
- ✅ 功能完整
- ❌ 外部脚本
- ❌ 需要解析日志

#### QuantlinkTrader

**内置监控**:
- ✅ 每 30 秒自动输出状态
- ✅ 结构化日志格式
- 🔜 **计划**: HTTP REST API 监控接口
- 🔜 **计划**: Prometheus 指标输出

**日志输出**:
```
[Main] ════════════════════════════════════════════════════════════
[Main] Periodic Status Update - 17:23:27
[Main] ────────────────────────────────────────────────────────────
[Main] Running:        true
[Main] Strategy ID:    92201
[Main] Mode:           live
[Main] Position:       10 (Long: 10, Short: 0)
[Main] P&L:            12500.50 (Realized: 10000.00, Unrealized: 2500.50)
[Main] ════════════════════════════════════════════════════════════
```

---

## 完整部署示例

### tbsrc 完整部署

```bash
# 1. 准备环境
cd /home/TradeBot/TradeBot_China/bin

# 2. 准备控制文件和模型文件
ls controls/ori/
control.ag2502.ag2504.par.txt.92201
control.al2502.al2503.par.txt.93201

ls models/
model.ag2502.ag2504.par.txt.92201
model.al2502.al2503.par.txt.93201

# 3. 运行 setup.py 生成配置
python setup.py

# 4. 启动策略
./start.comms.night.sh

# 5. 监控
tail -f log/log.control.ag2502.ag2504.par.txt.92201.20241226.night
../scripts/pnl_watch.sh
```

### QuantlinkTrader 完整部署

```bash
# 1. 准备环境
cd /Users/user/PWorks/RD/quantlink-trade-system/golang

# 2. 编译
go build -o QuantlinkTrader ./cmd/trader

# 3. 准备配置文件
ls config/
trader.ag2502.ag2504.yaml
trader.al2502.al2503.yaml
trader.rb2505.rb2510.yaml

# 4. 启动策略
./start_all_strategies.sh

# 5. 监控
tail -f log/trader.ag2502.ag2504.92201.log
# 或查看状态（日志中自动输出）
```

**对比**:
- ✅ **更简单**: 不需要 setup.py
- ✅ **更快**: 直接启动
- ✅ **更清晰**: 配置文件所见即所得

---

## 配置文件映射示例

### 完整对应关系

#### tbsrc 三个文件

**config_CHINA.92201.cfg**:
```ini
SHM_MD_KEY = 5592201
SHM_ORS_KEY = 5692201
EXCHANGE_NAME = SFE
SHM_MD_RESP_THREAD_CPU_AFFINITY = 18
STRATEGY_THREAD_CPU_AFFINITY = 20
TICK_SIZE = 1.0
CONTRACT_MULTIPLIER = 10
```

**control.ag2502.ag2504.par.txt.92201**:
```
ag_F_2_SFE ./models/model.ag2502.ag2504.par.txt.92201 SFE 16 TB_PAIR_STRAT 0100 0700 ag_F_4_SFE
```

**model.ag2502.ag2504.par.txt.92201**:
```
ag_F_2_SFE FUTCOM Dependant 0 MID_PX
ag_F_4_SFE FUTCOM Dependant 0 MID_PX
MAX_QUOTE_LEVEL 3
SIZE 4
MAX_SIZE 16
BEGIN_PLACE 5.006894
LONG_PLACE 7.510341
SHORT_PLACE 2.503447
UPNL_LOSS 100000
STOP_LOSS 100000
MAX_LOSS 100000
```

#### QuantlinkTrader 一个文件

**trader.ag2502.ag2504.yaml**:
```yaml
# 对应 config_CHINA.92201.cfg
system:
  strategy_id: "92201"              # 对应 --strategyID
  mode: "live"                      # 对应 --Live

engine:
  ors_gateway_addr: "localhost:50052"  # 对应 SHM_ORS_KEY 概念
  nats_addr: "nats://localhost:4222"   # 对应 SHM_MD_KEY 概念
  # Note: Go 使用 gRPC/NATS 代替共享内存

# 对应 control file
strategy:
  type: "pairwise_arb"              # 对应 TB_PAIR_STRAT
  symbols: ["ag2502", "ag2504"]     # 对应 ag_F_2_SFE, ag_F_4_SFE
  exchanges: ["SHFE", "SHFE"]       # 对应 SFE
  max_position_size: 16             # 对应 controlFile 中的 16

session:
  start_time: "09:00:00"            # 对应 0100 (UTC+8 09:00)
  end_time: "15:00:00"              # 对应 0700 (UTC+8 15:00)

# 对应 model file
strategy:
  parameters:
    order_size: 4                   # 对应 SIZE
    max_quote_level: 3              # 对应 MAX_QUOTE_LEVEL
    entry_zscore: 2.0               # 对应 BEGIN_PLACE 概念
    # ...

risk:
  stop_loss: 100000.0               # 对应 STOP_LOSS
  max_loss: 100000.0                # 对应 MAX_LOSS

logging:
  file: "./log/trader.ag2502.ag2504.92201.log"
```

---

## 总结

### 核心对齐

| 方面 | tbsrc | QuantlinkTrader | 对齐状态 |
|------|-------|-----------------|----------|
| **启动粒度** | 每交易对一个进程 | 每交易对一个进程 | ✅ **完全对齐** |
| **配置方式** | 3 层配置文件 | 单一 YAML | ✅ 更简洁 |
| **命令行参数** | 8 个参数 | 3-4 个参数 | ✅ 更简洁 |
| **进程管理** | 手动 PID 管理 | PID 文件 + 脚本 | ✅ 更自动化 |
| **日志管理** | 手动日期命名 | 自动轮转 | ✅ 更智能 |
| **监控** | 外部脚本 | 内置 + 外部 | ✅ 更集成 |

### 部署对比

**tbsrc**:
```bash
python setup.py  # 生成配置
./start.comms.night.sh  # 启动
```

**QuantlinkTrader**:
```bash
./start_all_strategies.sh  # 直接启动
```

✅ **更简单、更直接、更易维护**

### 结论

**QuantlinkTrader 的启动方式与 tbsrc TradeBot 完全对齐**：
- ✅ 每个交易对/策略实例一个进程
- ✅ 独立的配置文件
- ✅ 唯一的 strategy_id
- ✅ 后台运行
- ✅ PID 管理
- ✅ 日志管理

**改进之处**：
- ✅ 单一配置文件（vs 3 层配置）
- ✅ 标准 YAML 格式（vs 自定义格式）
- ✅ 更简洁的命令行
- ✅ 自动化的日志轮转
- ✅ 内置监控输出

---

**文档版本**: 1.0.0
**最后更新**: 2026-01-22
