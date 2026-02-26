# 实施任务

## 依赖和构建

- [x] 1. pom.xml 添加 Javalin + Jackson 依赖

## 数据模型

- [x] 2. DashboardSnapshot.java — 快照数据模型（对齐 Go snapshot.go，Jackson snake_case 序列化）

## API 服务

- [x] 3. ApiServer.java — Javalin HTTP 服务（7 个 REST 端点 + 静态文件）
- [x] 4. WebSocket 推送 — Javalin WsConfig 实现广播 hub
- [x] 5. SnapshotCollector.java — 每秒从 strategy 采集快照并推送

## 集成

- [x] 6. TraderMain 集成 — init() 启动 ApiServer，start() 启动 SnapshotCollector，shutdown() 关闭
- [x] 7. 命令处理 — cmdQueue 消费循环（activate/deactivate/squareoff/reload_thresholds）

## 前端

- [x] 8. 复制 Go 版 dashboard.html 到 resources/web/，配置 Javalin 静态文件

## 测试

- [x] 9. 编译验证 + 单元测试（184 tests pass）
