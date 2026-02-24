## 1. Overview 页面创建

- [x] 1.1 创建 `tbsrc-golang/web/overview.html`，替换旧文件，使用 Vue 3 CDN + Composition API
- [x] 1.2 实现端口扫描（9201-9210），通过 `ws://localhost:{port}/ws` 连接 trader
- [x] 1.3 解析 `{ type: "dashboard_update", data: DashboardSnapshot }` 消息更新卡片数据
- [x] 1.4 实现 ping/pong 心跳响应
- [x] 1.5 实现自动重连（已知 trader 断开后每 3 秒重试）和定期扫描（每 5 秒）

## 2. 策略卡片 UI

- [x] 2.1 实现策略卡片：Strategy ID、Active badge、Symbols、连接状态指示
- [x] 2.2 实现 PNL 显示：Net PNL、Realised、Unrealised（leg1 + leg2 汇总）
- [x] 2.3 实现 Spread 指标：Current、Deviation、T-Value、EWA Mean
- [x] 2.4 实现持仓和订单显示：Leg1/Leg2 Netpos、Exposure、Orders
- [x] 2.5 复用 dashboard.html CSS 变量、渐变 header、卡片布局、badge 样式

## 3. 操作控制

- [x] 3.1 实现操作按钮：Activate、Deactivate、Squareoff（带确认）、Reload Thresholds
- [x] 3.2 REST 调用 `POST /api/v1/strategy/{activate|deactivate|squareoff|reload-thresholds}`
- [x] 3.3 实现 Toast 通知显示操作结果
- [x] 3.4 断开连接时禁用操作按钮

## 4. 汇总栏和批量操作

- [x] 4.1 实现顶部汇总栏：Total Net PNL、Realised、Unrealised、Orders
- [x] 4.2 实现 Activate All / Deactivate All 批量操作按钮

## 5. Detail 导航

- [x] 5.1 实现 Detail 链接，新标签页打开 `http://localhost:{port}/dashboard.html`

## 6. Webserver 目录优先级

- [x] 6.1 更新 `tbsrc-golang/cmd/webserver/main.go` 目录查找顺序为 `web/` → `golang/web/`

## 7. 验证

- [x] 7.1 `build_deploy_new.sh --go` 编译并确认 overview.html 被复制到 deploy_new/web/
- [x] 7.2 启动 sim 模式网关 + 策略 92201，访问 `http://localhost:8080` 验证卡片显示和数据更新
- [x] 7.3 验证 Detail 链接跳转到 `http://localhost:9201/dashboard.html`
