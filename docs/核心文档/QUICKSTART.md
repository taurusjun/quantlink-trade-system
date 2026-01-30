# QuantlinkTrader 快速开始指南

**最后更新**: 2026-01-30  
**适用版本**: v1.0.0

---

## 📋 目录

- [概述](#概述)
- [前置要求](#前置要求)
- [快速启动](#快速启动)
- [使用示例](#使用示例)
- [配置说明](#配置说明)
- [常见问题](#常见问题)
- [进阶功能](#进阶功能)

---

## 概述

这是 QuantlinkTrader 最快速的启动方式，使用演示模式配置，特点：

✅ **一键启动/停止** - 无需手动启动多个组件  
✅ **订单量适中** - entry_zscore=1.0，适合观察测试  
✅ **风险限制宽松** - max_loss=500000，避免误触发紧急停止  
✅ **自动清理** - 每次启动前自动清理历史持仓和共享内存  
✅ **完整监控** - Dashboard + API + 日志

---

## 前置要求

### 1. 已编译的二进制文件

```bash
# Gateway 组件
gateway/build/md_simulator
gateway/build/md_gateway
gateway/build/ors_gateway
gateway/build/counter_bridge

# Trader 主程序
bin/trader
```

如未编译，请先执行：

```bash
# 编译 Gateway
cd gateway/build
cmake ..
make -j4

# 编译 Trader
cd ../../
go build -C golang -o bin/trader cmd/trader/main.go
```

### 2. 依赖服务

- **NATS Server**: 消息中间件（脚本会自动启动）
- **网络端口**: 4222 (NATS), 9201 (API), 50052 (gRPC)

---

## 快速启动

### 一键启动

```bash
./scripts/live/start_demo.sh
```

**启动流程**:
1. ✓ 检查二进制文件和配置
2. ✓ 清理旧进程和共享内存
3. ✓ **清理历史持仓数据** (重要！)
4. ✓ 启动 NATS Server
5. ✓ 启动行情组件 (md_simulator, md_gateway)
6. ✓ 启动订单路由 (ors_gateway)
7. ✓ 启动模拟成交 (counter_bridge)
8. ✓ 启动 Trader

**预期输出**:

```
╔═══════════════════════════════════════════════════════════╗
║  QuantlinkTrader - Demo Mode                              ║
║  模拟交易系统快速启动                                     ║
╚═══════════════════════════════════════════════════════════╝

[STEP] [0/7] Pre-flight checks...
[INFO] ✓ All binaries found
[INFO] ✓ Config file found

[STEP] [1/7] Cleaning up old processes and data...
[INFO] ✓ Old processes cleaned
[WARN] Found 5 position snapshot files
[INFO] ✓ Historical positions cleaned
[INFO] ✓ Shared memory cleaned

[STEP] [2/7] Starting NATS server...
[INFO] ✓ NATS started (PID: 12345)

...

═══════════════════════════════════════════════════════════
System Status
═══════════════════════════════════════════════════════════
  ✓ nats-server      (PID: 12345)
  ✓ md_simulator     (PID: 12346)
  ✓ md_gateway       (PID: 12347)
  ✓ ors_gateway      (PID: 12348)
  ✓ counter_bridge   (PID: 12349)
  ✓ trader           (PID: 12350)

═══════════════════════════════════════════════════════════
Access Information
═══════════════════════════════════════════════════════════
  📊 Dashboard:  http://localhost:9201/dashboard
  🔌 API:        http://localhost:9201/api/v1/
  📝 Logs:       tail -f log/trader.demo.log
```

### 一键停止

```bash
./scripts/live/stop_demo.sh
```

**停止流程**:
- ✓ 停止 Trader
- ✓ 停止 Counter Bridge
- ✓ 停止 ORS Gateway
- ✓ 停止行情组件
- ✓ 停止 NATS
- ✓ 清理共享内存

---

## 使用示例

### 1. 查看策略状态

```bash
curl http://localhost:9201/api/v1/strategy/status | jq .
```

**响应示例**:

```json
{
  "success": true,
  "message": "Strategy status retrieved",
  "data": {
    "strategy_id": "test_simple",
    "running": true,
    "active": false,
    "mode": "live",
    "symbols": ["ag2603", "ag2605"],
    "position": null,
    "conditions_met": true
  }
}
```

### 2. 激活策略

```bash
curl -X POST http://localhost:9201/api/v1/strategy/activate
```

**响应**:

```json
{
  "success": true,
  "message": "Strategy activated"
}
```

### 3. 查看实时订单

```bash
tail -f log/trader.demo.log | grep -E "Order sent|Trade"
```

**输出示例**:

```
2026/01/30 17:30:15 [StrategyEngine] Order sent: test_simple, OrderID: ORD_1769765415123_000001, Status: SUCCESS
2026/01/30 17:30:15 [StrategyEngine] Trade: ORD_1769765415123_000001, ag2603, BUY 2@8015.50
2026/01/30 17:30:15 [BaseStrategy:test_simple] ✅ EstimatedPosition UPDATED: Long=2, Short=0, Net=2
```

### 4. 查看持仓

```bash
curl http://localhost:9201/api/v1/positions | jq .
```

### 5. 停用策略

```bash
curl -X POST http://localhost:9201/api/v1/strategy/deactivate
```

---

## 配置说明

### 演示配置文件

**位置**: `config/trader.demo.yaml`

**关键参数**:

| 参数 | 值 | 说明 |
|------|-----|------|
| `entry_zscore` | 1.0 | 入场阈值（适中，容易触发） |
| `exit_zscore` | 0.3 | 出场阈值 |
| `order_size` | 2.0 | 每次下单2手 |
| `max_position_size` | 50 | 最大持仓50手 |
| `max_loss` | 500000 | 最大亏损50万（避免误触发） |
| `daily_loss_limit` | 500000 | 日亏损限制50万 |

### 参数调优

**如果订单太多**:
- 提高 `entry_zscore` (如 1.5)
- 减少 `order_size` (如 1.0)

**如果订单太少**:
- 降低 `entry_zscore` (如 0.8)
- 增加 `order_size` (如 3.0)

**如果触发风险限制**:
- 提高 `max_loss` (如 1000000)
- 提高 `daily_loss_limit` (如 1000000)

---

## 常见问题

### Q1: 启动后没有订单？

**检查清单**:

1. 策略是否激活？
   ```bash
   curl http://localhost:9201/api/v1/strategy/status | jq .data.active
   ```

2. 是否接收到行情？
   ```bash
   tail -f log/trader.demo.log | grep "Received market data"
   ```

3. 相关系数是否达标？
   ```bash
   tail -f log/trader.demo.log | grep "corr="
   ```

4. Z-Score 是否足够？
   ```bash
   tail -f log/trader.demo.log | grep "zscore="
   ```

**解决方案**: 降低 `entry_zscore` 到 0.5

### Q2: 系统触发紧急停止？

**原因**: 全局回撤超过 `max_loss` 限制

**解决方案**: 编辑 `config/trader.demo.yaml`，提高风险限制：

```yaml
risk:
  max_loss: 1000000.0        # 提高到100万
  daily_loss_limit: 1000000.0
```

### Q3: 端口被占用？

**检查端口**:

```bash
lsof -i :9201  # API 端口
lsof -i :4222  # NATS 端口
lsof -i :50052 # gRPC 端口
```

**解决方案**: 停止占用端口的进程，或修改配置文件中的端口号

### Q4: 历史持仓影响测试？

**症状**: 启动后立即有持仓数据

**原因**: 上次测试的持仓数据未清理

**解决方案**: 重新启动脚本会自动清理，或手动清理：

```bash
rm -f data/positions/*.json
```

### Q5: Dashboard 无法访问？

**检查 API 是否启动**:

```bash
curl http://localhost:9201/api/v1/health
```

**查看日志**:

```bash
tail -f log/trader.demo.log | grep API
```

---

## 进阶功能

### 1. 自定义配置

复制演示配置并修改：

```bash
cp config/trader.demo.yaml config/trader.mycustom.yaml
# 编辑配置...
./bin/trader -config config/trader.mycustom.yaml
```

### 2. 多策略模式

使用 `config/trader.test.yaml`（3个策略）：

```bash
./bin/trader -config config/trader.test.yaml
```

### 3. Dashboard 监控

访问 http://localhost:9201/dashboard 查看：

- 实时策略状态
- 持仓信息
- P&L 统计
- 订单历史

### 4. WebSocket 实时推送

```javascript
const ws = new WebSocket('ws://localhost:9201/api/v1/ws/dashboard');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Real-time update:', data);
};
```

### 5. 日志分析

```bash
# 查看所有订单
grep "Order sent" log/trader.demo.log

# 查看成交
grep "Trade" log/trader.demo.log

# 查看持仓更新
grep "EstimatedPosition UPDATED" log/trader.demo.log

# 查看信号生成
grep "Signal" log/trader.demo.log

# 查看风险告警
grep "ALERT\|EMERGENCY" log/trader.demo.log
```

---

## 脚本参考

### 启动脚本

**位置**: `scripts/live/start_demo.sh`

**功能**:
- 清理旧进程和数据
- 清理历史持仓 ⭐
- 启动所有组件
- 健康检查
- 输出访问信息

### 停止脚本

**位置**: `scripts/live/stop_demo.sh`

**功能**:
- 停止所有组件
- 清理共享内存
- 验证清理结果

---

## 相关文档

- **系统架构**: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
- **构建指南**: @docs/核心文档/BUILD_GUIDE.md
- **完整使用手册**: @docs/核心文档/USAGE.md
- **订单回报链路**: @docs/实盘/订单回报链路修复报告_2026-01-30-16_59.md

---

## 技术支持

遇到问题？查看：

1. **日志文件**: `log/trader.demo.log`
2. **TASKS.md**: 已知问题和修复进度
3. **测试报告**: `docs/测试报告/`

---

**🎉 现在开始你的量化交易之旅吧！**

