# 实施任务

## DashboardSnapshot 扩展

- [x] 1. DashboardSnapshot 新增 model_file/strategy_type/control_file 字段，collect() 中填充

## 后端聚合服务

- [x] 2. StrategyConnector.java — WebSocket 客户端连接各 trader /ws，实时接收 snapshot 推送，断线自动重连
- [x] 3. OverviewSnapshot.java — 聚合数据模型（策略列表 + 持仓 + 挂单 + 成交）
- [x] 4. OverviewServer.java — Javalin HTTP + WebSocket (port 8080)，REST API + 命令转发 + 推送驱动转发

## 前端页面

- [x] 5. overview.html 基础框架 — Vue 3 单文件，7 区域 CSS Grid 布局
- [x] 6. ① 顶部控制栏 — stopAll 按钮 + product/strategy/status 筛选下拉
- [x] 7. ② 策略列表表格 — 16 列完整实现，状态着色，操作按钮
- [x] 8. ③ Account Table — 右侧面板（表头占位）
- [x] 9. ④⑤⑥⑦ 底部 4 表格 — Spread Trades / Orders / Position / Fills

## 集成与测试

- [x] 10. 编译验证 + 启动 OverviewServer + 1-2 个 trader 实例浏览器验证
