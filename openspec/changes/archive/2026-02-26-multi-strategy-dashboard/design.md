# 设计: 多策略综合监控页面

## 决策: 后端聚合

后端 OverviewServer 负责采集和聚合所有 trader 实例数据，前端只做展示。

## 页面布局 — 7 区域

```
┌──────────────────────────────────────────────────────────────────────────┐
│ ① 顶部控制栏: [stopAll] product▼ strategy▼ status▼ [清除strategy]      │
├──────────────────────────────────────────────┬───────────────────────────┤
│ ② 策略列表表格 (主体)                        │ ③ Account Table (右侧)    │
│ Status|Alert|AT|Pro|ID|IP|ModelFile|Type|    │ Pro|Ex|Total|Avail|Margin │
│ Key|val|L1|L2|PNL|Time|Information|操作      │ |Risk                     │
│                                              │                           │
│                                              │                           │
├──────────────┬──────────────┬────────────────┴───────────────────────────┤
│③ Spread      │④ Orders      │⑤ Position    │⑥ Fills                     │
│ Trades       │ cxl All      │ Table         │                            │
│ModelFile|S   │Symbol|S|Qty  │Symbol|Pos     │Time|Symbol|S|Price         │
│|Qty|Spread   │|Price|Model  │|CXLRio|Pro    │|Qty|ID|Pro                 │
│|Time|Pro     │|ID|pro       │               │                            │
└──────────────┴──────────────┴───────────────┴────────────────────────────┘
```

## 决策: WebSocket 推送驱动（非轮询）

OverviewServer 作为 WebSocket **客户端**连接各 trader 的 `/ws` 端点，接收实时推送的 DashboardSnapshot，无需 HTTP 轮询。

优势:
- **实时**: trader 推送即到达，零额外延迟
- **高效**: 长连接无 HTTP 请求开销
- **断线感知**: WebSocket onClose 即时检测 trader 下线
- **架构一致**: 复用 trader 已有的 WebSocket 推送机制

## 后端架构

```
OverviewServer (port 8080, Javalin)
├── StrategyConnector (WebSocket 客户端, 连接各 trader)
│   ├── ws://localhost:9201/ws  ← 实时接收 DashboardSnapshot
│   ├── ws://localhost:9202/ws  ← 实时接收 DashboardSnapshot
│   └── ... 扫描 9201-9210 (每 5 秒重试未连接端口)
│   │
│   │  数据流: trader 推送 → StrategyConnector 接收 → 更新聚合状态
│   │  断线: onClose → 标记为"未连接" → 自动重连
│
├── DataAggregator (聚合数据, 由 StrategyConnector 推送驱动)
│   ├── 策略列表: 各 trader 的最新 snapshot
│   ├── 账户资金: TODO (CTP 查询)
│   ├── 持仓汇总: 聚合各 trader leg1/leg2 持仓
│   ├── 挂单汇总: 聚合各 trader orders
│   ├── 成交记录: 从 OrderHistoryTracker 获取
│   └── 期权表: TODO (期权策略支持后)
│
├── REST API
│   ├── GET /api/v1/overview       → 聚合快照
│   ├── GET /api/v1/positions      → 全局持仓
│   ├── GET /api/v1/all-orders     → 全局挂单
│   ├── GET /api/v1/all-fills      → 全局成交
│   ├── POST /api/v1/command/{port}/{action} → 转发控制命令
│   └── POST /api/v1/stop-all     → 停止所有策略
│
├── WebSocket /ws → 每次收到 trader 推送即转发聚合数据给前端
└── 静态文件 → overview.html
```

### 数据流

```
trader-1 ──ws push──→ StrategyConnector ──→ DataAggregator ──→ OverviewServer WS ──→ 前端
trader-2 ──ws push──→      (接收)              (聚合)            (转发推送)
trader-N ──ws push──→
```

## 9 区域数据来源

### ① 顶部控制栏
- stopAll → POST /api/v1/stop-all → 向所有 trader 发 deactivate + squareoff
- 筛选: 前端 JS 过滤表格行

### ② 策略列表表格（核心）
各列数据来源：

| 列 | 来源 | 说明 |
|----|------|------|
| Status | StrategyConnector WS 连接状态 | 运行中/无进程/未连接 |
| Alert | — | 预留 |
| AT | snapshot.active | 自动交易开关 |
| Pro | snapshot.leg1.symbol → 提取品种 | ag, cu 等 |
| ID | snapshot.strategy_id | 策略 ID |
| IP | StrategyConnector 记录的端口 | 端口号或 IP |
| ModelFile | 需扩展 snapshot | 模型文件名 |
| StrategyType | 需扩展 snapshot | TB_PAIR_STRAT 等 |
| Key | — | 预留 |
| val | snapshot.spread.is_valid | 有效性 |
| 1 (L1) | snapshot.leg1.netpos | 第一腿持仓 |
| 2 (L2) | snapshot.leg2.netpos | 第二腿持仓 |
| PNL | leg1.net_pnl + leg2.net_pnl | 总盈亏 |
| Time | snapshot.timestamp | 最后更新 |
| Information | spread + 关键指标 | 摘要信息 |
| 操作 | POST 转发 | 启动/停止/暂停/日志 |

### ③ Account Table（右侧）
- 当前: 空表（后续对接 CTP 资金查询）
- 列: Pro, Ex(交易所), TotalAsset, AvailCash, Margin, Risk

### ④ Spread Trades（底部左）
- 从各 trader 的已成交订单中提取价差成交
- 列: ModelFile, S(方向), Qty, Spread, Time, Pro

### ⑤ Orders（底部中左，全局挂单）
- 聚合各 trader 的 leg1.orders + leg2.orders
- 列: Symbol, S, Qty, Price, Modelfile, ID, pro
- cxl All: 批量撤单

### ⑥ Position Table（底部中右，全局持仓）
- 聚合各 trader 的 leg1/leg2 持仓
- 列: Symbol, Pos, CXLRio(撤单比), Pro

### ⑦ Fills（底部右，全局成交）
- 聚合各 trader 的 OrderHistoryTracker 中已成交订单
- 列: Time, Symbol, S, Price, Qty, ID, Pro

## DashboardSnapshot 扩展

需在现有 snapshot 中增加字段以支持策略列表列：

```java
// DashboardSnapshot 新增字段
@JsonProperty("model_file")    public String modelFile = "";
@JsonProperty("strategy_type") public String strategyType = "";
@JsonProperty("control_file")  public String controlFile = "";
```

## 新增文件

| 文件 | 说明 |
|------|------|
| `api/OverviewServer.java` | 聚合服务（Javalin, port 8080） |
| `api/OverviewSnapshot.java` | 聚合数据模型 |
| `api/StrategyConnector.java` | WebSocket 客户端，连接各 trader 接收实时推送 |
| `resources/web/overview.html` | 综合监控页面（Vue 3） |

## 依赖

- 已有: Javalin 6.4.0, Jackson 2.17.0
- 新增: java.net.http.HttpClient（JDK 内置 WebSocket 客户端，连接各 trader /ws）
