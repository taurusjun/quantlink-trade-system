# QuantlinkTrader 项目开发规则

## 项目概述

QuantlinkTrader 是一个高性能量化交易系统，采用 C++ 网关 + Golang 策略引擎的混合架构。

**关键文档**:
- 系统架构: @docs/CURRENT_ARCHITECTURE_FLOW.md
- 构建指南: @docs/BUILD_GUIDE.md
- 使用说明: @docs/USAGE.md
- 最新测试报告: @docs/QuantlinkTrader_端到端测试报告_2026-01-24-15_32.md

---

## 系统架构

### 核心组件

1. **C++ 网关层** (`gateway/`)
   - `md_simulator`: 模拟行情数据生成器
   - `md_gateway`: 行情网关（共享内存 → NATS）
   - `ors_gateway`: 订单路由服务（gRPC → 共享内存）
   - `counter_gateway`: 模拟成交网关

2. **Golang 策略层** (`golang/`)
   - `pkg/trader/`: 交易主程序
   - `pkg/strategy/`: 策略引擎
   - `pkg/portfolio/`: 组合管理
   - `pkg/risk/`: 风控模块

3. **通信机制**
   - POSIX 共享内存: C++ 网关间通信（低延迟）
   - NATS: 行情数据分发（md_gateway → golang_trader）
   - gRPC: 订单路由（golang_trader → ors_gateway）

### 数据流向

```
md_simulator → [SHM] → md_gateway → [NATS] → golang_trader → [gRPC] → ors_gateway → [SHM] → counter_gateway
```

---

## 代码风格规范

### C++ 代码 (`gateway/`)

- **风格指南**: 遵循 Google C++ Style Guide
- **命名规范**:
  - 类名: PascalCase (例如: `MarketDataGateway`)
  - 函数名: camelCase (例如: `processMarketData()`)
  - 成员变量: `m_` 前缀 (例如: `m_isRunning`)
  - 常量: UPPER_SNAKE_CASE (例如: `MAX_QUEUE_SIZE`)
- **头文件**:
  - 使用 `#pragma once` 而不是 include guards
  - 头文件包含顺序: 标准库 → 第三方库 → 项目头文件
- **共享内存**:
  - 使用 POSIX `shm_open` / `mmap`
  - 队列名格式: `ors_request`, `ors_response`, `md_queue`

### Golang 代码 (`golang/`)

- **风格指南**: 使用 `gofmt` 自动格式化
- **包命名**: 全小写，单数形式 (例如: `trader`, `strategy`, `risk`)
- **接口命名**:
  - 单方法接口以 `-er` 结尾 (例如: `Reader`, `Writer`)
  - 多方法接口使用描述性名称 (例如: `StrategyEngine`)
- **错误处理**:
  - 总是检查 error 返回值
  - 使用 `log.Printf` 记录错误，不要 panic（除非初始化失败）
- **日志格式**: `log.Printf("[模块名] 消息内容")`

### 配置文件 (`config/`)

- **格式**: YAML
- **命名**:
  - 生产配置: `trader.yaml`
  - 测试配置: `trader.test.yaml`
- **必填字段**:
  - `system.strategy_id`: 策略唯一标识
  - `strategy.symbols`: 交易品种列表
  - `engine.ors_gateway_addr`: ORS 网关地址
  - `engine.nats_addr`: NATS 服务地址

---

## 文档规范

### 文档存放位置

**规则 1: 文档目录结构**

- **通用文档**: 放在 `docs/` 根目录下
  - 适用于多模块协同、系统级别、架构级别的文档
  - 例如: 端到端测试报告、系统架构文档、部署指南

- **模块文档**: 放在 `docs/` 下对应的模块子目录
  - `docs/gateway/`: C++ 网关相关文档
  - `docs/golang/`: Golang 策略引擎相关文档
  - `docs/config/`: 配置相关文档
  - 例如: gateway 特定的性能优化文档、golang 包设计文档

**目录结构示例**:
```
quantlink-trade-system/
├── docs/                                    # 文档根目录
│   ├── QuantlinkTrader_端到端测试报告_2026-01-24-15_32.md  # 通用文档
│   ├── 系统_架构设计_2026-01-20-10_00.md                    # 通用文档
│   ├── 部署_生产环境指南_2026-01-21-14_30.md                # 通用文档
│   ├── gateway/                             # Gateway 模块文档
│   │   ├── gateway_共享内存优化_2026-01-15-09_20.md
│   │   └── gateway_性能测试_2026-01-16-11_45.md
│   └── golang/                              # Golang 模块文档
│       ├── golang_策略引擎设计_2026-01-18-13_15.md
│       └── golang_风控模块实现_2026-01-19-16_00.md
├── gateway/                                 # C++ 网关代码
│   └── src/
└── golang/                                  # Golang 策略代码
    └── pkg/
```

### 文档命名规范

**规则 2: 文档命名格式**

**格式**: `模块_摘要_YYYY-MM-DD-HH_mm.md`

**组成部分**:
- **模块**: 文档所属模块或主题（小写或驼峰）
  - 单模块: `gateway`, `golang`, `config`, `strategy`, `risk` 等
  - 多模块/系统级: `QuantlinkTrader`, `系统`, `项目`, `部署` 等
- **摘要**: 简短描述文档内容（2-5 个字，中文）
- **时间戳**: `YYYY-MM-DD-HH_mm` 格式（24 小时制）

**命名示例**:

```bash
# ✅ 正确示例
docs/QuantlinkTrader_端到端测试报告_2026-01-24-15_32.md    # 通用文档
docs/系统_架构设计_2026-01-20-10_00.md                       # 通用文档
docs/部署_生产环境指南_2026-01-21-14_30.md                   # 通用文档

docs/gateway/gateway_共享内存优化_2026-01-15-09_20.md       # Gateway 模块文档
docs/gateway/gateway_性能测试报告_2026-01-16-11_45.md       # Gateway 模块文档

docs/golang/golang_策略引擎设计_2026-01-18-13_15.md         # Golang 模块文档
docs/golang/golang_风控模块实现_2026-01-19-16_00.md         # Golang 模块文档
docs/golang/strategy_配对套利策略_2026-01-22-10_30.md       # Golang 模块文档

# ❌ 错误示例
docs/test_report.md                                         # 缺少时间戳
docs/EndToEndTest_2026-01-24.md                            # 使用英文，缺少时分
gateway/docs/gateway_共享内存优化.md                         # 错误位置
docs/系统架构_20260120.md                                    # 时间格式不正确
```

### 文档内容规范

**规则 3: 使用中文编写**

- **文档正文**: 必须使用中文
- **标题**: 使用中文
- **注释**: 使用中文
- **例外情况**:
  - 代码片段: 保持原始语言（C++/Golang/Shell 等）
  - 命令示例: 保持原始命令
  - 技术术语: 可保留英文，但首次出现时提供中文说明
  - 文件路径: 保持原始路径
  - URL 链接: 保持原始链接

**示例**:

```markdown
# ✅ 正确示例

## 策略引擎设计

策略引擎（Strategy Engine）负责接收市场数据并生成交易信号。

### 核心接口

\`\`\`go
type StrategyEngine interface {
    Start() error
    Stop() error
}
\`\`\`

### 运行方式

执行以下命令启动策略引擎：

\`\`\`bash
./bin/trader -config config/trader.yaml
\`\`\`

---

# ❌ 错误示例

## Strategy Engine Design

The strategy engine is responsible for receiving market data...
```

### 文档模板

**标准文档模板**:

```markdown
# 模块名_文档标题

**文档日期**: YYYY-MM-DD
**作者**: [作者名]
**版本**: v1.0
**相关模块**: [模块列表]

---

## 概述

[简要描述文档目的和背景]

## 详细内容

### 章节 1

[内容...]

### 章节 2

[内容...]

## 总结

[总结要点]

## 参考资料

- 相关文档1: @docs/xxx.md
- 相关文档2: @gateway/docs/xxx.md

---

**最后更新**: YYYY-MM-DD HH:mm
```

### 文档维护

**规则 4: 文档生命周期**

- **创建文档**: 重大功能、架构变更、测试报告时创建
- **更新文档**: 不修改原文件，而是创建新版本（带新时间戳）
- **废弃文档**: 可以移动到 `docs/archive/` 目录
- **文档索引**: 在 `docs/README.md` 中维护文档索引

**文档索引示例** (`docs/README.md`):

```markdown
# QuantlinkTrader 文档索引

## 系统文档（通用）

- [端到端测试报告](QuantlinkTrader_端到端测试报告_2026-01-24-15_32.md) - 2026-01-24
- [系统架构设计](CURRENT_ARCHITECTURE_FLOW.md)
- [构建指南](BUILD_GUIDE.md)

## 模块文档

### Gateway 模块
- [共享内存优化](gateway/gateway_共享内存优化_2026-01-15-09_20.md)
- [性能测试报告](gateway/gateway_性能测试报告_2026-01-16-11_45.md)

### Golang 模块
- [策略引擎设计](golang/golang_策略引擎设计_2026-01-18-13_15.md)
- [风控模块实现](golang/golang_风控模块实现_2026-01-19-16_00.md)
```

---

## 开发工作流

### 构建系统

```bash
# C++ 网关编译
cd gateway
mkdir -p build && cd build
cmake ..
make -j4

# Golang 编译（输出到项目根目录 bin/）
cd golang
go build -o ../bin/trader cmd/trader/main.go

# 或者从项目根目录构建
go build -C golang -o bin/trader cmd/trader/main.go
```

### 运行测试

**端到端测试** (推荐):
```bash
# 1. 启动 NATS
nats-server &

# 2. 运行完整链路测试
./test_full_chain.sh

# 3. 激活策略（等待 5 秒启动完成）
sleep 5
curl -X POST http://localhost:9201/api/v1/strategy/activate \
  -H "Content-Type: application/json" \
  -d '{"strategy_id": "test_92201"}'

# 4. 监控订单生成
tail -f log/trader.test.log | grep "Order sent"

# 5. 停止测试
pkill -f md_simulator
pkill -f md_gateway
pkill -f ors_gateway
pkill -f counter_gateway
pkill -f "trader -config"

# 6. 清理共享内存
ipcs -m | grep user | awk '{print $2}' | xargs ipcrm -m
```

**单元测试**:
```bash
# Golang 单元测试
cd golang
go test ./pkg/...

# C++ 单元测试（如果有）
cd gateway/build
ctest
```

### 调试方法

**查看日志**:
```bash
# 主日志
tail -f log/trader.test.log

# 订单记录
grep "Order sent" log/trader.test.log

# 策略统计
grep "Stats:" log/trader.test.log | tail -20

# 市场数据接收
grep "Received market data" log/trader.test.log
```

**检查进程状态**:
```bash
ps aux | grep -E "md_simulator|md_gateway|ors_gateway|counter_gateway|trader"
```

**检查共享内存**:
```bash
ipcs -m
```

**检查 NATS 消息**:
```bash
nats sub "md.>"
```

---

## 配置管理

### 测试配置 vs 生产配置

**测试配置** (`config/trader.test.yaml`):
```yaml
session:
  start_time: "00:00:00"        # 全天运行
  end_time: "23:59:59"
  auto_activate: false           # 需要手动激活

strategy:
  parameters:
    entry_zscore: 0.5            # 降低阈值便于测试
    exit_zscore: 0.2
```

**生产配置** (`config/trader.yaml`):
```yaml
session:
  start_time: "09:00:00"        # 实际交易时段
  end_time: "15:00:00"
  auto_activate: false           # 推荐手动激活

strategy:
  parameters:
    entry_zscore: 2.0            # 更保守的阈值
    exit_zscore: 0.5
```

### 关键配置项说明

- **entry_zscore**: Z-Score 入场阈值
  - 测试环境: 0.5（容易触发）
  - 生产环境: 2.0（更保守）

- **auto_activate**: 自动激活策略
  - 推荐设置为 `false`，手动激活更安全

- **max_position_size**: 最大持仓
  - 根据账户资金和风险承受能力设置

---

## 重要约定

### NATS 主题格式

- **发布格式**: `md.{exchange}.{symbol}`
  - 例如: `md.SHFE.ag2502`, `md.SHFE.ag2504`

- **订阅格式**: `md.*.{symbol}`
  - 使用通配符支持多交易所

### 订单 ID 格式

- 格式: `ORD_{timestamp_nano}`
- 例如: `ORD_1769239216860813000`

### 共享内存队列命名

- 请求队列: `ors_request`
- 响应队列: `ors_response`
- 行情队列: `md_queue`

---

## 常见问题排查

### 问题：无订单生成

**检查清单**:
1. 策略是否已激活？
   ```bash
   curl http://localhost:9201/api/v1/strategy/status
   ```

2. 是否接收到市场数据？
   ```bash
   grep "Received market data" log/trader.test.log
   ```

3. 相关系数是否达标？
   ```bash
   grep "corr=" log/trader.test.log | tail -5
   ```

4. Z-Score 是否超过阈值？
   ```bash
   grep "zscore=" log/trader.test.log | tail -5
   ```

5. 阈值是否过高？
   - 测试环境建议 `entry_zscore: 0.5`

### 问题：共享内存错误

```bash
# 清理所有共享内存段
ipcs -m | grep user | awk '{print $2}' | xargs ipcrm -m

# 重启相关进程
./test_full_chain.sh
```

### 问题：NATS 连接失败

```bash
# 检查 NATS 是否运行
ps aux | grep nats-server

# 重启 NATS
pkill nats-server
nats-server &
```

### 问题：gRPC 连接超时

```bash
# 检查 ORS Gateway 是否运行
ps aux | grep ors_gateway

# 检查端口是否监听
lsof -i :50052
```

---

## 安全规范

### 禁止事项

- ❌ 在代码中硬编码密钥、密码
- ❌ 提交敏感配置文件（使用 `.gitignore`）
- ❌ 在生产环境使用 `auto_activate: true`
- ❌ 跳过风险检查
- ❌ 在不了解的情况下修改共享内存结构

### 推荐实践

- ✅ 使用环境变量或外部配置管理敏感信息
- ✅ 测试新策略时先用小仓位
- ✅ 定期备份配置和日志
- ✅ 代码审查关注风控逻辑
- ✅ 提交前运行完整测试

---

## 文件组织结构

```
quantlink-trade-system/
├── gateway/              # C++ 网关代码
│   ├── src/             # 源文件
│   ├── include/         # 头文件
│   └── build/           # 编译产物（不提交）
├── golang/              # Golang 策略代码
│   ├── cmd/             # 主程序入口
│   ├── pkg/             # 业务逻辑包
│   └── internal/        # 内部包
├── config/              # 配置文件
│   ├── trader.yaml      # 生产配置
│   └── trader.test.yaml # 测试配置
├── bin/                 # 可执行文件（不提交）
├── log/                 # 日志文件（不提交）
├── test_logs/           # 测试日志（不提交）
├── docs/                # 文档
└── .claude/             # Claude Code 规则
```

### .gitignore 重点

```gitignore
# 编译产物
gateway/build/
bin/
*.o
*.so

# 日志
log/
test_logs/
*.log

# 临时文件
*.swp
*.tmp
.DS_Store

# 敏感配置
config/*.local.yaml
.env
```

---

## 性能要求

### 延迟指标

- 共享内存读写: < 1ms
- NATS 消息传输: < 5ms
- 策略计算: < 10ms
- 订单发送: < 20ms
- **端到端延迟**: < 50ms

### 资源限制

- CPU 使用: 单进程 < 20%
- 内存占用: 单进程 < 100MB
- 网络流量: < 10MB/min

---

## Git 工作流

### 分支策略

- `main`: 生产稳定版本
- `develop`: 开发主分支
- `feature/*`: 新功能开发
- `bugfix/*`: Bug 修复
- `hotfix/*`: 紧急修复

### 提交规范

使用 Conventional Commits:

```
feat: 添加配对套利策略
fix: 修复共享内存泄漏问题
docs: 更新端到端测试文档
refactor: 重构订单路由逻辑
test: 添加策略引擎单元测试
chore: 更新依赖版本
```

### 提交前检查

1. ✅ 代码已格式化（C++/Golang）
2. ✅ 通过编译
3. ✅ 通过单元测试
4. ✅ 更新相关文档
5. ✅ 检查 .gitignore（不提交日志、二进制文件）

---

## 部署检查清单

### 上线前验证

- [ ] 完整端到端测试通过
- [ ] 配置文件已切换到生产配置
- [ ] 策略参数已调整到保守值（entry_zscore ≥ 2.0）
- [ ] 风控参数已设置合理值
- [ ] 日志级别设置正确（info 或 warn）
- [ ] 资源监控已就绪
- [ ] 回滚方案已准备

### 监控指标

- 订单成功率
- 策略信号频率
- 系统延迟
- CPU/内存使用
- 错误日志数量

---

## 联系方式

**系统维护**: 参考 @docs/README.md

**问题反馈**: 创建 Issue 或提交 PR

---

**最后更新**: 2026-01-24
**文档版本**: v1.1
