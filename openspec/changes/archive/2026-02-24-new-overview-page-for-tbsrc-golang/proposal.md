## Why

旧 overview.html 是从 `golang/` 目录复制来的，使用不兼容的 WebSocket 路径 (`/api/v1/ws/dashboard`) 和多策略/端口数据模型。tbsrc-golang 的 trader 使用 `/ws` 路径、`DashboardSnapshot` 数据格式、每端口一个策略模型，旧页面无法正常工作。需要全新编写 overview 页面以匹配当前架构。

## What Changes

- 替换 `tbsrc-golang/web/overview.html`，使用正确的 WebSocket 路径 `/ws` 和 `{ type: "dashboard_update", data: DashboardSnapshot }` 消息格式
- 端口扫描 9201-9210，每个端口一个策略卡片
- 策略卡片显示：Strategy ID、Active 状态、Symbols、PNL（Realised/Unrealised/Net）、Spread 指标、持仓、订单数
- 顶部汇总栏：Total PNL、Realised、Unrealised、总订单数
- 操作按钮：Activate、Deactivate、Squareoff、Reload Thresholds，调用 `POST /api/v1/strategy/{action}`
- Detail 链接跳转到 `http://localhost:{port}/dashboard.html`
- 更新 `tbsrc-golang/cmd/webserver/main.go` 目录查找顺序，优先使用 `web/` 目录

## Capabilities

### New Capabilities

- `overview-page`: 多策略总览页面，端口扫描连接多个 trader，聚合显示 PNL 和策略状态，提供批量操作和单策略控制

### Modified Capabilities

（无已有 spec 需要修改）

## Impact

- `tbsrc-golang/web/overview.html` — 完全重写
- `tbsrc-golang/cmd/webserver/main.go` — 目录查找顺序调整
- `scripts/build_deploy_new.sh` — 无需修改，已有 `*.html` glob 自动包含新文件
- 依赖：Vue 3 CDN（与 dashboard.html 一致）
