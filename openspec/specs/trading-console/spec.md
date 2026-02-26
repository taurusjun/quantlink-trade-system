# Spec: 交易控制台

## 需求

### R1: REST API

与 Go 版 `pkg/api/handlers.go` 完全一致的 7 个端点:

- `GET /api/v1/health` — 返回 `{"success": true, "data": {"ws_clients": N}}`
- `GET /api/v1/status` — 返回 `{"success": true, "data": DashboardSnapshot}`
- `GET /api/v1/orders` — 返回 `{"success": true, "data": {"leg1": [...], "leg2": [...]}}`
- `POST /api/v1/strategy/activate` — 激活策略
- `POST /api/v1/strategy/deactivate` — 停用策略（不平仓）
- `POST /api/v1/strategy/squareoff` — 平仓
- `POST /api/v1/strategy/reload-thresholds` — 热加载阈值

所有响应格式: `{"success": bool, "message": "...", "data": ...}`
CORS: `Access-Control-Allow-Origin: *`

### R2: WebSocket 实时推送

- 端点 `/ws`
- 每 ~1 秒推送 `{"type": "dashboard_update", "timestamp": "...", "data": DashboardSnapshot}`
- 每 30 秒心跳 `{"type": "ping", "timestamp": "..."}`
- 支持多客户端并发连接

### R3: DashboardSnapshot 数据模型

JSON 字段名与 Go `pkg/api/snapshot.go` 完全一致（snake_case），包含:
- 策略状态: strategy_id, active, account, exposure
- 价差: spread.current, spread.avg_spread, spread.avg_ori, spread.t_value, spread.deviation, spread.is_valid, spread.alpha
- 双腿: leg1/leg2 各含 symbol, exchange, 行情(6), 持仓(3), PNL(6), 交易统计(6), 动态阈值(6), 挂单(4), 状态标志(3), orders[]

### R4: 命令执行

POST 控制命令在 TraderMain 主线程异步执行:
- activate: 设置 strategy.active + firstStrat.active + secondStrat.active = true
- deactivate: 设置 active = false
- squareoff: 调用 strategy.handleSquareoff()
- reload_thresholds: 调用 reloadThresholds()

### R5: 前端页面

复用 Go 版 `web/dashboard.html`，通过 Javalin 静态文件提供服务。
访问 `http://localhost:9201/` 可打开控制台。

### R6: 生命周期

- ApiServer 在 TraderMain.init() 末尾启动
- TraderMain.shutdown() 中优雅关闭 ApiServer
- SnapshotCollector 在 start() 后启动定时采集
