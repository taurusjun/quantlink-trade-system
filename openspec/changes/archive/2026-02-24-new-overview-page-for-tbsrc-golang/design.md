## Context

当前 `tbsrc-golang/web/overview.html` 是从旧 `golang/` 项目复制来的文件，使用 `/api/v1/ws/dashboard` WebSocket 路径和多策略/端口数据模型（一个 trader 进程管理多个策略）。tbsrc-golang 架构改为每端口一个策略，trader 使用 `/ws` WebSocket 路径，发送 `{ type: "dashboard_update", data: DashboardSnapshot }` 格式消息。overview 页面需要全新编写以匹配新架构。

overview 由独立 webserver 进程在 8080 端口提供服务，扫描 9201-9210 端口连接各个 trader。

## Goals / Non-Goals

**Goals:**
- 创建与 tbsrc-golang trader 兼容的多策略总览页面
- 复用 dashboard.html 的视觉风格（CSS 变量、渐变 header、卡片、badge）
- 支持端口扫描自动发现 trader、自动重连
- 提供聚合 PNL 汇总和单策略操作按钮
- 单页 HTML + Vue 3 CDN，无构建步骤

**Non-Goals:**
- 不实现历史 PNL 图表或时序数据展示
- 不实现用户认证或权限控制
- 不修改 trader 的 WebSocket 或 REST API

## Decisions

### 1. 单页 HTML + Vue 3 CDN
**选择**: 与 dashboard.html 一致，使用 Vue 3 CDN + Composition API
**理由**: 保持技术栈统一，无需构建工具，部署简单（一个 HTML 文件）
**替代方案**: React/Svelte SPA — 增加构建复杂度，不值得

### 2. WebSocket 端口扫描模式
**选择**: 页面加载时并发尝试连接 9201-9210 所有端口，连接成功的显示卡片，失败的静默忽略
**理由**: 无需配置即可自动发现 trader，与旧 overview 扫描模式一致
**替代方案**: 从中心注册中心获取 trader 列表 — 当前无此基础设施

### 3. 每端口一个策略卡片
**选择**: 直接使用 `DashboardSnapshot` 数据渲染卡片，`leg1 + leg2` 汇总 PNL
**理由**: 与 tbsrc-golang 一对一 trader/策略模型匹配

### 4. webserver 目录查找优先 web/
**选择**: 将 `web/` 放在 `golang/web/` 之前
**理由**: deploy_new 目录下只有 `web/`（由 build_deploy_new.sh 复制），应优先匹配

## Risks / Trade-offs

- **[端口范围固定]** → 硬编码 9201-9210，如需更多策略需修改 HTML 常量。当前 10 个端口足够。
- **[跨域请求]** → overview 在 8080 端口，REST 命令发往 9201+ 端口，依赖 trader 的 CORS 支持。trader 已设置 `Access-Control-Allow-Origin: *`。
- **[无持久化状态]** → 页面刷新后需重新扫描，无历史数据。符合当前需求。
