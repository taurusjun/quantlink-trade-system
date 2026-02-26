## Why

Java trader 进程目前只有信号控制（SIGUSR1/SIGUSR2/SIGTSTP/SIGINT），无法实时监控策略运行状态，也无法通过 Web 界面操作。Go 版已有完整的 REST API + WebSocket + 前端页面（pkg/api/ + web/），Java 需要对齐实现，提供交易控制台。

## What Changes

- 新增 Javalin HTTP/WebSocket 服务，内嵌到 TraderMain 进程（port 9201）
- 新增 DashboardSnapshot 数据模型（对齐 Go pkg/api/snapshot.go）
- 新增 REST API（status/health/orders/activate/deactivate/squareoff/reload-thresholds）
- 新增 WebSocket 实时推送（每秒推送 DashboardSnapshot）
- 复用 Go 版 web/dashboard.html 前端页面（Vue 3 单文件，无需构建）
- pom.xml 新增 Javalin 依赖

## Capabilities

### New Capabilities
- `trading-console`: Java 内嵌 HTTP/WebSocket 交易控制台，包含实时监控（持仓/PnL/价差/行情）和控制操作（激活/停止/平仓/热加载阈值）

### Modified Capabilities
<!-- 无需修改现有 spec -->

## Impact

- **新增依赖**: Javalin (~1MB)，添加到 pom.xml
- **TraderMain.java**: init() 中启动 Javalin 服务
- **新增文件**: `api/` 包（ApiServer, DashboardSnapshot, WebSocketHandler）
- **前端**: 复用 Go 版 web/dashboard.html，适配 Java 端口
- **端口**: 9201（与 Go 版一致，同一时间只运行一个版本）
