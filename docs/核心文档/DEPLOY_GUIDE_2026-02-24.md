# QuantLink Trade System - 部署指南

**文档日期**: 2026-02-24
**版本**: v1.0
**适用架构**: SysV MWMR SHM 直连架构（tbsrc-golang v2）

---

## 概述

QuantLink Trade System 采用 C++/Golang 混合架构，通过 SysV 共享内存（MWMR 队列）实现超低延迟的进程间通信。Go trader 直接读写 SysV SHM，与 C++ ORS 使用完全相同的队列格式。

---

## 1. 系统架构

### 1.1 数据流

```
行情路径:
  md_shm_feeder ──→ [SysV MWMR SHM, key=0x1001] ──→ Go trader (直接读取)

订单路径:
  Go trader ──→ [SysV MWMR SHM, key=0x2001] ──→ counter_bridge ──→ CTP/Simulator
                                                        │
  Go trader ←── [SysV MWMR SHM, key=0x3001] ←──────────┘ (回报)

客户端注册:
  Go trader ←→ [SysV SHM, key=0x4001] ←→ counter_bridge (ClientStore 原子分配)
```

### 1.2 核心进程

| 进程 | 语言 | 说明 |
|------|------|------|
| `md_shm_feeder` | C++ | 行情注入器（支持 simulator / CTP 模式），写入 SysV MWMR SHM |
| `counter_bridge` | C++ | 统一成交网关（支持 simulator / CTP 插件），读取订单 SHM、写入回报 SHM |
| `trader` | Go | 策略引擎，直接读写 SysV SHM（行情、订单、回报） |
| `webserver` | Go | Web 监控（Overview 页面，端口 8080） |

### 1.3 SysV SHM Key 分配

| Key | 用途 | 写入方 | 读取方 |
|-----|------|--------|--------|
| `0x1001` (4097) | 行情队列（MarketUpdateNew） | md_shm_feeder | trader |
| `0x2001` (8193) | 订单请求队列（RequestMsg） | trader | counter_bridge |
| `0x3001` (12289) | 订单回报队列（ResponseMsg） | counter_bridge | trader |
| `0x4001` (16385) | ClientStore（客户端 ID 分配） | trader / counter_bridge | trader / counter_bridge |

---

## 2. 编译

### 2.1 一键编译部署

```bash
# 完整编译（C++ + Go），输出到 deploy_new/
./scripts/build_deploy_new.sh

# 仅编译 Go 组件
./scripts/build_deploy_new.sh --go

# 仅编译 C++ 组件
./scripts/build_deploy_new.sh --cpp

# 清理后重新编译
./scripts/build_deploy_new.sh --clean
```

### 2.2 手动编译

```bash
# C++ 网关
cd gateway && mkdir -p build && cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
make -j$(nproc) md_shm_feeder counter_bridge

# Go 策略引擎
cd tbsrc-golang
go build -o ../bin/trader ./cmd/trader/main.go
go build -o ../bin/webserver ./cmd/webserver/main.go

# Linux 交叉编译（从 macOS）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/trader_linux ./cmd/trader/main.go
```

### 2.3 Go 单元测试

```bash
cd tbsrc-golang && go test ./pkg/...
```

---

## 3. 目录结构

### 3.1 deploy_new/（编译产物 + 运行时）

由 `build_deploy_new.sh` 自动生成，可直接部署到目标服务器。

```
deploy_new/
├── bin/                          # 可执行文件
│   ├── trader                    # Go 策略引擎
│   ├── webserver                 # Go Web 监控
│   ├── md_shm_feeder             # C++ 行情 SHM 注入器
│   ├── counter_bridge            # C++ 统一成交网关
│   ├── backtest                  # Go 回测引擎（可选）
│   ├── backtest_optimize         # Go 回测优化器（可选）
│   ├── md_simulator              # C++ 行情模拟器（legacy，已被 md_shm_feeder simulator 模式替代）
│   ├── md_gateway                # C++ 行情网关（legacy，SHM 直连后不再使用）
│   ├── ors_gateway               # C++ 订单网关（legacy，SHM 直连后不再使用）
│   └── ctp_md_gateway            # C++ CTP 行情网关（legacy）
├── config/                       # 配置文件
│   ├── trader.92201.yaml         # 策略 92201 配置
│   ├── trader.92202.yaml         # 策略 92202 配置
│   ├── simulator.yaml            # 模拟器配置
│   └── ctp/                      # CTP 配置
│       ├── ctp_md.secret.yaml    # CTP 行情账号（gitignored）
│       └── ctp_td.secret.yaml    # CTP 交易账号（gitignored）
├── controls/                     # 策略控制文件（C++ .ctrl 格式）
│   ├── day/                      # 日盘
│   └── night/                    # 夜盘
├── models/                       # 策略模型文件（C++ .model 格式）
├── data/                         # 运行时数据（daily_init、positions）
├── scripts/                      # 运行脚本
│   ├── start_gateway.sh          # 启动网关（sim/ctp）
│   ├── start_strategy.sh         # 启动策略（按 ID）
│   ├── start_all.sh              # 一键启动（网关 + 所有策略）
│   └── stop_all.sh               # 停止所有
├── web/                          # Web 静态资源
├── lib/                          # 动态库（CTP framework 等）
├── log/                          # 日志
└── ctp_flow/                     # CTP 流文件目录
```

### 3.2 data_new/（持久配置模板）

存放配置模板和模型文件，编译时自动合并到 `deploy_new/`。

```
data_new/
├── config/                       # 配置文件模板
├── controls/                     # 策略控制文件
├── models/                       # 策略模型文件
└── data/                         # 初始数据
```

**设计原则**：`deploy_new/` 包含编译产物（代码变动时重建），`data_new/` 包含持久配置（手动维护）。`build_deploy_new.sh` 自动将 `data_new/` 合并到 `deploy_new/`。

---

## 4. 配置文件

### 4.1 策略配置（每策略一个文件）

命名格式：`config/trader.{strategy_id}.yaml`

```yaml
# config/trader.92201.yaml 示例
system:
  strategy_id: 92201
  strategy_type: "TB_PAIR_STRAT"

shm:
  request_key: 0x2001        # 订单请求 SHM key
  request_size: 4096          # 队列容量
  response_key: 0x3001        # 订单回报 SHM key
  response_size: 4096
  md_key: 0x1001              # 行情 SHM key
  md_size: 4096
  client_store_key: 0x4001    # ClientStore SHM key

strategy:
  symbols:
    - ag2603
    - ag2605
  parameters:
    begin_place: 0.5
    long_place: 2.0
    short_place: -2.0
    begin_remove: 0.2
    alpha: 0.01
    max_quote_level: 3
    supporting_orders: 5
    size: 1
    max_size: 10

session:
  start_time: "09:00:00"
  end_time: "15:00:00"

dashboard:
  port: 9201                  # 策略独立 Dashboard 端口
```

### 4.2 CTP 配置

```yaml
# config/ctp/ctp_md.secret.yaml
broker_id: "9999"
user_id: "your_user_id"
password: "your_password"
front_addr: "tcp://180.168.146.187:10131"

# config/ctp/ctp_td.secret.yaml
broker_id: "9999"
user_id: "your_user_id"
password: "your_password"
auth_code: "your_auth_code"
app_id: "your_app_id"
front_addr: "tcp://180.168.146.187:10130"
```

### 4.3 模拟器配置

```yaml
# config/simulator.yaml
symbols:
  - ag2603
  - ag2605
tick_interval_ms: 500
```

---

## 5. 启动流程

### 5.1 模拟环境

```bash
cd deploy_new

# 1. 启动网关层（md_shm_feeder simulator + counter_bridge simulator + webserver）
./scripts/start_gateway.sh sim

# 2. 启动策略（可启动多个）
./scripts/start_strategy.sh 92201
./scripts/start_strategy.sh 92202

# 或一键启动
./scripts/start_all.sh sim
```

### 5.2 CTP 实盘

```bash
cd deploy_new

# 确保 CTP 配置文件存在
ls config/ctp/ctp_md.secret.yaml
ls config/ctp/ctp_td.secret.yaml

# 1. 启动网关层（md_shm_feeder ctp + counter_bridge ctp + webserver）
./scripts/start_gateway.sh ctp

# 2. 启动策略
./scripts/start_strategy.sh 92201
```

### 5.3 策略前台调试

```bash
# 前台运行（日志直接输出到终端）
./scripts/start_strategy.sh 92201 --fg
```

---

## 6. 监控

### 6.1 Web 监控

| 地址 | 说明 |
|------|------|
| `http://localhost:8080` | Overview 总览页面（webserver） |
| `http://localhost:9201/overview` | 策略 92201 Overview |
| `http://localhost:9201/dashboard` | 策略 92201 Dashboard |

### 6.2 日志

```bash
# 查看策略日志
tail -f deploy_new/nohup.out.92201

# 查看网关日志
tail -f deploy_new/log/md_shm_feeder.$(date +%Y%m%d).log
tail -f deploy_new/log/counter_bridge.$(date +%Y%m%d).log
tail -f deploy_new/log/webserver.$(date +%Y%m%d).log
```

### 6.3 进程状态

```bash
ps aux | grep -E 'trader|md_shm_feeder|counter_bridge|webserver'
```

### 6.4 共享内存状态

```bash
ipcs -m
```

---

## 7. 停止

```bash
cd deploy_new
./scripts/stop_all.sh
```

停止顺序：
1. 先发 SIGTERM 给 trader（等待 graceful shutdown 保存 daily_init）
2. 等待最多 10 秒，超时则 SIGKILL
3. 停止 counter_bridge、md_shm_feeder、webserver
4. 清理 SysV 共享内存段

---

## 8. 平台差异

### 8.1 macOS vs Linux

| 项目 | macOS | Linux (CentOS) |
|------|-------|----------------|
| SHM 队列大小 | 2048（系统限制） | 65536 |
| SHM 总大小限制 | ~4MB 默认 | 可配置到 GB 级 |
| CTP 库 | Framework 格式 | .so 格式 |
| Go 编译 | 原生 | 交叉编译或原生 |

### 8.2 调整 macOS SHM 限制（可选）

```bash
# 查看当前限制
sysctl kern.sysv.shmmax
sysctl kern.sysv.shmall

# 临时调整（重启后失效）
sudo sysctl -w kern.sysv.shmmax=67108864    # 64MB
sudo sysctl -w kern.sysv.shmall=16384       # 16384 pages
```

---

## 9. 服务器部署

### 9.1 部署步骤

```bash
# 1. 本地编译
./scripts/build_deploy_new.sh

# 2. 打包（如需 Linux 交叉编译）
cd tbsrc-golang
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../deploy_new/bin/trader ./cmd/trader/main.go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../deploy_new/bin/webserver ./cmd/webserver/main.go

# 3. 上传到目标服务器
scp -r deploy_new/ user@server:/opt/quantlink/

# 4. 在服务器上启动
ssh user@server
cd /opt/quantlink/deploy_new
./scripts/start_gateway.sh ctp
./scripts/start_strategy.sh 92201
```

### 9.2 检查清单

- [ ] CTP 配置文件已部署（`config/ctp/*.secret.yaml`）
- [ ] 策略配置参数已调整为生产值
- [ ] `ctp_flow/` 目录存在
- [ ] SysV SHM 限制满足需求
- [ ] 时区设置正确（CST / Asia/Shanghai）

---

## 10. 故障排查

### 无行情数据

```bash
# 检查 md_shm_feeder 进程
ps aux | grep md_shm_feeder

# 检查 SHM 是否创建
ipcs -m | grep 0x1001

# 检查日志
tail -f log/md_shm_feeder.*.log
```

### 无订单发出

```bash
# 检查 trader 进程
ps aux | grep trader

# 检查策略是否激活
grep -i "active\|activate" nohup.out.92201

# 检查 counter_bridge 是否运行
ps aux | grep counter_bridge
```

### 共享内存错误

```bash
# 清理所有 SysV 共享内存
ipcs -m | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {}

# 重启所有进程
./scripts/stop_all.sh
./scripts/start_gateway.sh sim
```

---

## 参考资料

- 架构设计: `docs/系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md`
- MWMR 技术规格: `docs/系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md`
- counter_bridge 改造: `docs/系统分析/counter_bridge_MWMR改造方案_2026-02-13-19_00.md`
- 编译脚本: `scripts/build_deploy_new.sh`

---

**最后更新**: 2026-02-24
