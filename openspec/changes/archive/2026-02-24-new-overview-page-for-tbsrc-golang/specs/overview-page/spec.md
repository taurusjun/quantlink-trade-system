## ADDED Requirements

### Requirement: Port scanning and auto-discovery
Overview 页面 SHALL 在加载时并发扫描端口 9201-9210，通过 WebSocket 连接 `ws://localhost:{port}/ws` 自动发现运行中的 trader。

#### Scenario: Trader running on port 9201
- **WHEN** 页面加载且 9201 端口有 trader 运行
- **THEN** 页面 SHALL 成功建立 WebSocket 连接并显示该 trader 的策略卡片

#### Scenario: No traders running
- **WHEN** 页面加载且 9201-9210 端口均无 trader 运行
- **THEN** 页面 SHALL 显示 "No Traders Found" 空状态提示

#### Scenario: Periodic re-scan
- **WHEN** 页面已加载
- **THEN** 页面 SHALL 每 5 秒重新扫描所有端口以发现新启动的 trader

### Requirement: WebSocket auto-reconnect
Overview 页面 SHALL 在已知 trader 断开连接后自动重连。

#### Scenario: Trader restart
- **WHEN** 已连接的 trader 断开连接
- **THEN** 页面 SHALL 每 3 秒尝试重连，并在卡片上显示断开状态

### Requirement: Dashboard data consumption
Overview 页面 SHALL 解析 `{ type: "dashboard_update", data: DashboardSnapshot }` 格式的 WebSocket 消息。

#### Scenario: Receive dashboard update
- **WHEN** 收到 `dashboard_update` 消息
- **THEN** 页面 SHALL 更新对应端口的策略卡片数据

#### Scenario: Respond to ping
- **WHEN** 收到 `{ type: "ping" }` 消息
- **THEN** 页面 SHALL 回复 `{ type: "pong" }`

### Requirement: Strategy card display
每个连接成功的 trader SHALL 显示一张策略卡片，包含以下信息：

- Strategy ID (`strategy_id`)
- Active 状态 badge (`active`)
- Symbols (`leg1.symbol` / `leg2.symbol`)
- PNL: Net PNL、Realised、Unrealised（`leg1 + leg2` 汇总）
- Spread 指标: Current、Deviation、T-Value、EWA Mean
- 持仓: Leg1 Netpos、Leg2 Netpos、Exposure
- 订单数: `leg1.orders.length + leg2.orders.length`

#### Scenario: Active strategy card
- **WHEN** trader 报告 `active: true`
- **THEN** 卡片 SHALL 显示绿色 "Active" badge 和绿色边框

#### Scenario: Inactive strategy card
- **WHEN** trader 报告 `active: false`
- **THEN** 卡片 SHALL 显示灰色 "Inactive" badge 和蓝色边框

### Requirement: Aggregate stats bar
页面顶部 SHALL 显示所有策略的汇总统计。

#### Scenario: Multiple traders connected
- **WHEN** 多个 trader 连接成功
- **THEN** 页面 SHALL 显示 Total Net PNL、Total Realised、Total Unrealised、Total Orders（所有 trader 合计）

### Requirement: Strategy control actions
每张策略卡片 SHALL 提供以下操作按钮：

- Activate: `POST /api/v1/strategy/activate`
- Deactivate: `POST /api/v1/strategy/deactivate`
- Squareoff: `POST /api/v1/strategy/squareoff`（需确认对话框）
- Reload Thresholds: `POST /api/v1/strategy/reload-thresholds`

#### Scenario: Activate strategy
- **WHEN** 用户点击 "Activate" 按钮
- **THEN** 页面 SHALL 发送 POST 请求到 `http://localhost:{port}/api/v1/strategy/activate` 并显示结果 toast

#### Scenario: Squareoff with confirmation
- **WHEN** 用户点击 "Squareoff" 按钮
- **THEN** 页面 SHALL 弹出确认对话框，确认后才发送请求

#### Scenario: Disconnected trader
- **WHEN** trader 连接断开
- **THEN** 所有操作按钮 SHALL 被禁用

### Requirement: Batch operations
汇总栏 SHALL 提供批量操作按钮。

#### Scenario: Activate all
- **WHEN** 用户点击 "Activate All"
- **THEN** 页面 SHALL 对所有已连接且未激活的策略发送 activate 命令

#### Scenario: Deactivate all
- **WHEN** 用户点击 "Deactivate All"
- **THEN** 页面 SHALL 弹出确认对话框，确认后对所有已连接且已激活的策略发送 deactivate 命令

### Requirement: Detail navigation
每张策略卡片 SHALL 提供 Detail 链接。

#### Scenario: Click detail link
- **WHEN** 用户点击 "Detail" 链接
- **THEN** 浏览器 SHALL 在新标签页打开 `http://localhost:{port}/dashboard.html`

### Requirement: Webserver directory priority
webserver MUST 优先查找 `web/` 目录，其次 `golang/web/`。

#### Scenario: web/ directory exists
- **WHEN** webserver 启动且 `web/` 目录存在
- **THEN** webserver SHALL 使用 `web/` 目录提供静态文件

#### Scenario: Only golang/web/ exists
- **WHEN** webserver 启动且仅 `golang/web/` 目录存在
- **THEN** webserver SHALL 回退到 `golang/web/` 目录
