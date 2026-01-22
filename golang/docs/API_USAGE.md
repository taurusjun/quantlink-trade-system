# QuantlinkTrader HTTP REST API 使用指南

## 概述

QuantlinkTrader 提供两种策略控制方式：

1. **Unix 信号控制**（传统方式，对应 tbsrc）
   - SIGUSR1: 激活策略
   - SIGUSR2: 停止策略并平仓

2. **HTTP REST API**（现代方式，P1 实现）
   - 提供基于 HTTP 的 RESTful API
   - 易于集成到 Web 界面、监控系统等
   - 跨平台支持更好

## API 配置

在 trader 配置文件中启用 API：

```yaml
api:
  enabled: true           # 启用 API 服务器
  port: 9201             # API 端口
  host: "localhost"      # 绑定地址
```

推荐端口分配规则：
- 策略 92201 → API 端口 9201
- 策略 93201 → API 端口 9301
- 策略 41231 → API 端口 4101

## API 端点

### 基础 URL

```
http://localhost:{port}/api/v1
```

### 1. 激活策略

**POST** `/strategy/activate`

激活策略，开始交易（对应 SIGUSR1 信号）。

**请求示例：**
```bash
curl -X POST http://localhost:9201/api/v1/strategy/activate
```

**响应示例：**
```json
{
  "success": true,
  "message": "Strategy activated successfully",
  "data": {
    "strategy_id": "92201",
    "active": true,
    "running": true
  }
}
```

### 2. 停止策略（平仓）

**POST** `/strategy/deactivate`

停止策略并平仓（对应 SIGUSR2 信号）。

**请求示例：**
```bash
curl -X POST http://localhost:9201/api/v1/strategy/deactivate
```

**响应示例：**
```json
{
  "success": true,
  "message": "Strategy deactivated successfully (squareoff initiated)",
  "data": {
    "strategy_id": "92201",
    "active": false,
    "flatten": true
  }
}
```

### 3. 获取策略状态

**GET** `/strategy/status`

获取详细的策略状态信息。

**请求示例：**
```bash
curl -X GET http://localhost:9201/api/v1/strategy/status
```

**响应示例：**
```json
{
  "success": true,
  "message": "Strategy status retrieved",
  "data": {
    "strategy_id": "92201",
    "running": true,
    "active": true,
    "mode": "live",
    "symbols": ["ag2502", "ag2504"],
    "position": {
      "long": 8,
      "short": 8,
      "net": 0
    },
    "pnl": {
      "realized": 12500.50,
      "unrealized": 2300.00,
      "total": 14800.50
    },
    "risk": {
      "current_drawdown": 1200.00,
      "max_drawdown": 10000.00
    },
    "uptime": "2h35m12s",
    "details": {
      "flatten_mode": false,
      "exit_requested": false,
      "cancel_pending": false,
      "strategy_type": "pairwise_arb",
      "max_position": 16,
      "max_exposure": 500000.0
    }
  }
}
```

### 4. 获取 Trader 状态

**GET** `/trader/status`

获取整体 trader 状态（包括所有组件）。

**请求示例：**
```bash
curl -X GET http://localhost:9201/api/v1/trader/status
```

**响应示例：**
```json
{
  "success": true,
  "message": "Trader status retrieved",
  "data": {
    "running": true,
    "strategy_id": "92201",
    "mode": "live",
    "strategy": { ... },
    "position": { ... },
    "pnl": { ... },
    "risk": { ... }
  }
}
```

### 5. 健康检查

**GET** `/health`

快速健康检查端点。

**请求示例：**
```bash
curl -X GET http://localhost:9201/api/v1/health
```

**响应示例：**
```json
{
  "success": true,
  "message": "Healthy",
  "data": {
    "status": "ok",
    "trader": true,
    "api_server": true,
    "strategy_id": "92201",
    "mode": "live"
  }
}
```

## 使用 apiControl.sh 脚本

提供便捷的命令行工具 `apiControl.sh`：

### 激活策略
```bash
./apiControl.sh activate 92201
./apiControl.sh activate 93201 9301    # 指定端口
```

### 停止策略
```bash
./apiControl.sh deactivate 92201
```

### 查询状态
```bash
./apiControl.sh status 92201
./apiControl.sh trader-status 92201
./apiControl.sh health 92201
```

## API vs Unix 信号对比

| 功能 | Unix 信号 | HTTP API |
|------|----------|----------|
| 激活策略 | `kill -SIGUSR1 <pid>` | `POST /strategy/activate` |
| 停止策略 | `kill -SIGUSR2 <pid>` | `POST /strategy/deactivate` |
| 查询状态 | ❌ 不支持 | `GET /strategy/status` |
| 健康检查 | ❌ 不支持 | `GET /health` |
| 跨平台 | ❌ Unix/Linux 只 | ✓ 所有平台 |
| Web 集成 | ❌ 困难 | ✓ 容易 |
| 监控集成 | ❌ 困难 | ✓ 容易 |
| 需要 PID | ✓ 需要 | ❌ 不需要 |

## 使用场景

### 1. 脚本批量控制
```bash
# 使用 Unix 信号（快速，适合本地）
./startAllTrades.sh

# 使用 API（适合远程，跨平台）
for port in 9201 9301 4101; do
    curl -X POST http://localhost:${port}/api/v1/strategy/activate
done
```

### 2. 监控告警集成
```bash
# Prometheus 风格的健康检查
while true; do
    STATUS=$(curl -s http://localhost:9201/api/v1/health | jq -r '.data.status')
    if [ "$STATUS" != "ok" ]; then
        # 发送告警
        alert "Strategy 92201 is unhealthy"
    fi
    sleep 60
done
```

### 3. Web 控制界面
```javascript
// JavaScript 示例
async function activateStrategy(strategyId, port) {
    const response = await fetch(`http://localhost:${port}/api/v1/strategy/activate`, {
        method: 'POST'
    });
    const result = await response.json();
    return result;
}

async function getStrategyStatus(strategyId, port) {
    const response = await fetch(`http://localhost:${port}/api/v1/strategy/status`);
    const result = await response.json();
    return result.data;
}
```

### 4. Python 自动化
```python
import requests

class StrategyController:
    def __init__(self, strategy_id, port):
        self.base_url = f"http://localhost:{port}/api/v1"

    def activate(self):
        response = requests.post(f"{self.base_url}/strategy/activate")
        return response.json()

    def deactivate(self):
        response = requests.post(f"{self.base_url}/strategy/deactivate")
        return response.json()

    def get_status(self):
        response = requests.get(f"{self.base_url}/strategy/status")
        return response.json()['data']

# 使用示例
controller = StrategyController("92201", 9201)
controller.activate()
status = controller.get_status()
print(f"PNL: {status['pnl']['total']}")
```

## 错误处理

### 错误响应格式
```json
{
  "success": false,
  "error": "Failed to start strategy: strategy already running"
}
```

### 常见错误

1. **连接失败**
   - 检查 trader 是否运行
   - 检查 API 是否在配置中启用
   - 检查端口是否正确

2. **405 Method Not Allowed**
   - 检查 HTTP 方法是否正确（POST vs GET）

3. **500 Internal Server Error**
   - 检查 trader 日志获取详细错误信息

## 安全建议

1. **生产环境**：
   - 使用防火墙限制访问
   - 考虑添加认证（JWT、API Key）
   - 使用 HTTPS（添加反向代理如 nginx）

2. **本地开发**：
   - 绑定到 localhost 避免外部访问
   - 使用不同端口避免冲突

## 与 tbsrc 对比总结

| tbsrc | QuantlinkTrader |
|-------|----------------|
| 只支持 Unix 信号 | 支持 Unix 信号 + HTTP API |
| `startTrade.pl` | `startTrade.sh` + `apiControl.sh activate` |
| `stopTrade.pl` | `stopTrade.sh` + `apiControl.sh deactivate` |
| 无状态查询 | API 提供完整状态查询 |
| 本地控制 | 本地 + 远程控制 |

## 完整工作流示例

```bash
# 1. 启动 trader（配置中已启用 API）
./runTrade.sh 92201

# 2. 检查健康状态
./apiControl.sh health 92201

# 3. 激活策略（两种方式任选其一）
./startTrade.sh 92201                  # Unix 信号方式
# 或
./apiControl.sh activate 92201         # API 方式

# 4. 监控策略状态
./apiControl.sh status 92201

# 5. 停止策略（两种方式任选其一）
./stopTrade.sh 92201                   # Unix 信号方式
# 或
./apiControl.sh deactivate 92201       # API 方式
```
