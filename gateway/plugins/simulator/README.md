# Simulator Plugin

模拟交易所插件，用于策略开发、测试和回测。

## 特性

- ✅ **立即成交模式** - 订单提交后快速成交（可配置延迟）
- ✅ **完整持仓管理** - 今昨仓分离、开平仓逻辑
- ✅ **风险控制** - 持仓限制、资金检查、日亏损限制
- ✅ **HTTP API** - 统计信息、账户查询、持仓查询
- ✅ **线程安全** - 多线程环境下安全运行
- ✅ **订单簿** - 完整的订单簿数据结构
- ✅ **撮合引擎** - 灵活的撮合引擎框架

## 快速开始

### 1. 编译

```bash
cd gateway/build
cmake .. -DBUILD_SIMULATOR_PLUGIN=ON
make counter_bridge
```

### 2. 配置

编辑 `config/simulator/simulator.yaml`:

```yaml
mode: "immediate"

account:
  initial_balance: 1000000.0    # 初始资金
  commission_rate: 0.0003       # 手续费率
  margin_rate: 0.10             # 保证金率

matching:
  accept_delay_ms: 50           # 接受延迟
  fill_delay_ms: 100            # 成交延迟
  slippage_ticks: 1.0           # 滑点

risk:
  max_position_per_symbol: 1000
  max_daily_loss: 100000.0
```

### 3. 启动

```bash
# 启动完整系统
./scripts/live/start_simulator.sh

# 或手动启动
./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml
```

### 4. 使用

```bash
# 激活策略
curl -X POST http://localhost:9201/api/v1/strategy/activate

# 查询统计
curl http://localhost:8080/simulator/stats | jq .

# 查询账户
curl http://localhost:8080/simulator/account | jq .

# 查询持仓
curl http://localhost:8080/positions | jq .
```

### 5. 停止

```bash
./scripts/live/stop_all.sh
```

## API 端点

### GET /simulator/stats
获取统计信息（订单数、成交数、连接状态）

### GET /simulator/account
获取账户信息（余额、可用资金、保证金、盈亏）

### GET /positions
获取所有持仓信息

### GET /health
健康检查

## 文件结构

```
simulator/
├── include/
│   ├── simulator_config.h      # 配置管理
│   ├── simulator_plugin.h      # 主插件类
│   ├── order_book.h            # 订单簿
│   └── matching_engine.h       # 撮合引擎
└── src/
    ├── simulator_config.cpp
    ├── simulator_plugin.cpp
    ├── order_book.cpp
    └── matching_engine.cpp
```

## 核心类

### SimulatorPlugin
主插件类，实现 `ITDPlugin` 接口。

**主要方法**:
- `Initialize()` - 初始化
- `Login()` - 登录
- `SendOrder()` - 发送订单
- `CancelOrder()` - 撤销订单
- `QueryAccount()` - 查询账户
- `QueryPositions()` - 查询持仓

### OrderBook
订单簿实现，维护买卖盘深度。

**主要方法**:
- `AddOrder()` - 添加订单
- `RemoveOrder()` - 删除订单
- `GetBestBid()` - 最优买价
- `GetBestAsk()` - 最优卖价
- `GetSnapshot()` - 订单簿快照

### MatchingEngine
撮合引擎，处理订单匹配逻辑。

**主要方法**:
- `AddOrder()` - 添加订单到引擎
- `OnMarketData()` - 处理行情数据
- `GetOrderBook()` - 获取订单簿

## 测试

### 端到端测试
```bash
./scripts/test/e2e/test_simulator_e2e.sh
```

### 手动测试
```bash
# 1. 启动系统
./scripts/live/start_simulator.sh

# 2. 等待启动
sleep 10

# 3. 激活策略
curl -X POST http://localhost:9201/api/v1/strategy/activate

# 4. 查看日志
tail -f log/trader.log | grep "Order"

# 5. 检查持仓
curl http://localhost:8080/positions | jq .

# 6. 停止系统
./scripts/live/stop_all.sh
```

## 配置说明

### mode
- `immediate` - 立即成交模式（默认）
- `market_driven` - 行情驱动模式（需要行情源）

### account
- `initial_balance` - 初始资金（元）
- `commission_rate` - 手续费率（小数）
- `margin_rate` - 保证金率（小数）

### matching
- `accept_delay_ms` - 订单接受延迟（毫秒）
- `fill_delay_ms` - 订单成交延迟（毫秒）
- `slippage_ticks` - 滑点（tick 数）

### risk
- `max_position_per_symbol` - 单品种最大持仓
- `max_daily_loss` - 最大日亏损（元）

## 性能

- 订单处理: >1000 orders/sec
- 查询响应: <5ms
- 内存占用: ~50MB
- CPU 占用: <5%

## 限制

1. 数据仅保存在内存，重启后丢失
2. 行情驱动模式需要额外配置
3. 订单簿深度 API 未完全实现

## 文档

- [完整实施报告](../../docs/功能实现/模拟交易所_完整实施报告_2026-01-30-15_00.md)
- [系统架构](../../docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md)
- [使用指南](../../docs/核心文档/USAGE.md)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可

与主项目相同
