# Spec: Overview 综合监控页面

## 需求

### R1: 后端聚合服务

OverviewServer 独立运行在端口 8080，负责：
- 作为 WebSocket 客户端连接各 trader 的 `/ws`（扫描 9201-9210），实时接收 DashboardSnapshot 推送
- 聚合策略列表、持仓、挂单、成交数据
- 每次收到 trader 推送即转发聚合数据给前端 WebSocket
- REST API 转发控制命令到对应 trader
- 断线自动重连（每 5 秒重试未连接端口）

### R2: 7 区域页面布局

页面包含以下 7 个区域，布局与原系统一致（去掉 PyBot 和 Option）：

**① 顶部控制栏**
- `stopAll` 红色按钮 — 停止所有策略
- `product` 下拉筛选
- `strategy` 下拉筛选
- `status` 下拉筛选
- `清除strategy` 按钮

**② 策略列表表格**（主体区域，占页面左侧）
列: Status | Alert | AT | Pro | ID | IP | ModelFile | StrategyType | Key | val | 1(L1持仓) | 2(L2持仓) | PNL | Time | Information | 操作(启动/停止/暂停/日志)

行为：
- 每行一个策略实例
- Status 着色: 运行中(绿)、无进程(灰)、未连接(黄)
- L1/L2 持仓着色: 正数绿、负数红、零灰
- PNL 着色: 正数绿、负数红
- 操作按钮: 启动/停止/暂停/日志

**③ Account Table**（右侧）
列: Broker | AccountID | TotalAsset | AvailCash | Margin | Risk(%) | ClosePnL | PosPnL

OverviewServer SHALL 每 10 秒通过 HTTP GET 查询 `http://localhost:8082/account` 获取资金数据。

查询结果 SHALL 缓存为 `AccountRow` 列表，在每次聚合 OverviewSnapshot 时合并到 `accounts` 字段。

Account Table SHALL 通过 Vue 数据绑定渲染 `overview.accounts` 数组。

Risk(%) SHALL 计算为 `margin / balance * 100`，超过 50% 时以红色高亮显示。

当 counter_bridge 未启动或查询失败时，Account Table SHALL 显示 "Waiting for counter_bridge..." 占位文字。

**④ Spread Trades**（底部左）
列: ModelFile | S(方向) | Qty | Spread | Time | Pro
- 数据来源: 各 trader 的已成交订单，计算价差

**⑤ Orders**（底部中左）
列: Symbol | S | Qty | Price | Modelfile | ID | pro
- `cxl All` 按钮 — 批量撤单
- strategy/symbol 下拉筛选
- 数据来源: 聚合各 trader 的活跃挂单

**⑥ Position Table**（底部中右）
列: Symbol | Pos | CXLRio | Pro
- product/symbol/strategy 下拉筛选
- 数据来源: 聚合各 trader 的 leg1/leg2 持仓

**⑦ Fills**（底部右）
列: Time | Symbol | S(方向) | Price | Qty | ID | Pro
- 数据来源: 聚合各 trader 的已成交订单（OrderHistoryTracker）

### R3: 数据刷新

- OverviewServer 作为 WS 客户端连接各 trader `/ws`，实时接收推送
- 每次收到任一 trader 推送 → 重新聚合 → 推送给前端 WebSocket
- 前端收到推送后更新所有 9 个区域
- 心跳: 30 秒 ping
- 断线: trader 下线 → onClose → 标记"未连接" → 5 秒后自动重连

### R4: 控制命令

- 单策略操作: POST 到 OverviewServer → 转发到对应 trader 端口
- stopAll: 向所有已连接 trader 发送 deactivate + squareoff
- cxl All: 向所有已连接 trader 发送撤单命令（需扩展 trader API）

### R5: DashboardSnapshot 扩展

现有 snapshot 新增字段：
- `model_file`: 模型文件名（从 controlFile 获取）
- `strategy_type`: 策略类型（TB_PAIR_STRAT 等）
- `control_file`: 控制文件路径

### R6: 样式要求

- 紧凑表格风格，对齐原系统
- 深色/浅色行交替
- 状态 badge 着色
- 表格固定表头，内容可滚动
- 响应式: 最小宽度 1400px
