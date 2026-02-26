## Context

Overview 页面已实现策略列表、持仓、挂单、成交的聚合展示，但 Account Table 仅为占位。counter_bridge (C++) 已有完整的 `ITDPlugin::QueryAccount()` 接口和 `AccountInfo` 结构体，CTP 和 Simulator 插件均已实现。OverviewServer (Java) 运行在 8080 端口，counter_bridge 原本也在 8080，存在端口冲突。

## Goals / Non-Goals

**Goals:**
- counter_bridge 暴露通用 `/account` HTTP 端点，兼容 CTP 和 Simulator
- OverviewServer 定时查询并推送资金数据到前端
- Account Table 展示真实资金信息（余额、可用、保证金、风险度、盈亏）
- 解决 counter_bridge 与 OverviewServer 的 8080 端口冲突

**Non-Goals:**
- 多账户聚合（当前只支持单 broker plugin）
- 资金数据持久化或历史查询
- Account Table 的控制操作（如出入金）

## Decisions

### D1: counter_bridge HTTP 端口改为 8082

**选择**: 8082
**理由**: OverviewServer 占用 8080，counter_bridge 需要独立端口。8082 与 8080 相近，便于记忆。8081 保留给可能的 trader API 扩展。
**替代方案**: 让 OverviewServer 换端口 — 但 8080 是 web 惯例，改 counter_bridge 更合理。

### D2: 通用 `/account` 端点而非仅 `/simulator/account`

**选择**: 新增 `GET /account`，遍历 `g_brokers` 找第一个已登录的插件调用 `QueryAccount()`
**理由**: 一个端点同时支持 CTP 和 Simulator，Java 端无需区分模式。保留 `/simulator/account` 做兼容。

### D3: OverviewServer 轮询而非 WebSocket/推送

**选择**: `ScheduledExecutorService` 每 10 秒 HTTP GET
**理由**: 资金数据变化频率低（秒级足够），HTTP 轮询实现简单可靠。counter_bridge 已有 httplib HTTP server，无需额外 WebSocket 支持。
**替代方案**: counter_bridge 主动推送 — 增加复杂度，收益不大。

### D4: AccountRow 作为 OverviewSnapshot 内部类

**选择**: 在 `OverviewSnapshot` 中定义 `AccountRow` 静态内部类
**理由**: 与 `StrategyRow`、`PositionRow` 等保持一致的模式，Jackson 自动序列化。

## Risks / Trade-offs

- **[端口冲突]** counter_bridge 8080→8082 是 breaking change → 仅影响内部脚本，已同步更新 `build_deploy_java.sh`
- **[counter_bridge 未启动]** OverviewServer 查询失败 → 捕获 `ConnectException` 静默忽略，Account Table 显示 "Waiting for counter_bridge..."
- **[CTP 查询延迟]** CTP `QueryAccount` 是同步 RPC → 10 秒轮询间隔足够缓冲，不影响交易路径
