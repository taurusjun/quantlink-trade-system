# 设计: Java 交易控制台

## 决策: HTTP 框架选型

**选择 Javalin** — 极轻量嵌入式 HTTP 框架（~1MB），原生支持 WebSocket，无 Spring Boot 依赖。API 风格简洁（`app.get("/path", handler)`），适合嵌入到现有 TraderMain 进程。

## 决策: 完全对齐 Go API

Java 版 API 端点、JSON 格式、WebSocket 消息结构完全对齐 Go 版（`pkg/api/`），前端 HTML 可直接复用。

### REST API 端点

| 方法 | 路径 | 对齐 Go |
|------|------|---------|
| GET | `/api/v1/health` | `handleHealth` |
| GET | `/api/v1/status` | `handleStatus` |
| GET | `/api/v1/orders` | `handleOrders` |
| POST | `/api/v1/strategy/activate` | `handleActivate` |
| POST | `/api/v1/strategy/deactivate` | `handleDeactivate` |
| POST | `/api/v1/strategy/squareoff` | `handleSquareoff` |
| POST | `/api/v1/strategy/reload-thresholds` | `handleReloadThresholds` |

### WebSocket

- 端点: `/ws`
- 推送消息格式: `{"type": "dashboard_update", "timestamp": "...", "data": DashboardSnapshot}`
- 心跳: 每 30 秒发送 `{"type": "ping"}`
- 快照推送间隔: ~1 秒

### DashboardSnapshot JSON 格式

完全对齐 Go `pkg/api/snapshot.go`:
- `timestamp`, `strategy_id`, `active`, `account`, `exposure`
- `spread`: current, avg_spread, avg_ori, t_value, deviation, is_valid, alpha
- `leg1`/`leg2`: symbol, exchange, 行情(6), 持仓(3), PNL(6), 交易统计(6), 动态阈值(6), 挂单(4), 状态标志(3), orders[]

## 架构

```
TraderMain 进程 (单 JVM)
├── Connector (SHM 轮询线程)
├── PairwiseArbStrategy (策略逻辑)
├── ApiServer (Javalin, port 9201)
│   ├── REST handlers (7 个端点)
│   ├── WebSocket hub (广播快照)
│   └── 静态文件 (dashboard.html)
└── SnapshotCollector (定时器线程, 每秒采集)
```

### 数据流

1. SnapshotCollector 每秒从 strategy 字段采集 DashboardSnapshot
2. ApiServer.updateSnapshot() 原子存储 + WebSocket 广播
3. REST `/api/v1/status` 返回最新快照
4. POST 命令 → cmdQueue → TraderMain 主线程执行

### 命令执行

Go 使用 channel，Java 使用 `LinkedBlockingQueue<String>`:
- `activate` → `strategy.active = true; firstStrat.active = true; secondStrat.active = true`
- `deactivate` → `strategy.active = false; firstStrat.active = false; secondStrat.active = false`
- `squareoff` → `strategy.handleSquareoff()`
- `reload_thresholds` → `reloadThresholds()` (已有方法)

## 新增文件

| 文件 | 说明 |
|------|------|
| `api/ApiServer.java` | Javalin HTTP + WebSocket 服务 |
| `api/DashboardSnapshot.java` | 快照数据模型（对齐 Go snapshot.go） |
| `api/SnapshotCollector.java` | 定时采集快照 |

## 依赖

pom.xml 新增:
- `io.javalin:javalin:6.4.0`
- `com.fasterxml.jackson.core:jackson-databind:2.17.0`（Javalin JSON 序列化需要）

## 前端

直接复用 Go 版 `web/dashboard.html`（Vue 3 单文件 HTML），仅需:
1. 复制到 `tbsrc-java/src/main/resources/web/dashboard.html`
2. Javalin `staticFiles.add("/web", Location.CLASSPATH)` 提供服务
