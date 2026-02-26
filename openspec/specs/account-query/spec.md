## ADDED Requirements

### Requirement: counter_bridge 通用资金查询端点

counter_bridge SHALL 在 HTTP 端口 8082 上提供 `GET /account` 端点，返回当前可用 broker plugin 的资金信息。

端点 SHALL 遍历 `g_brokers` 注册表，找到第一个已登录（`IsLoggedIn() == true`）的插件，调用 `QueryAccount()` 获取资金数据。

响应 JSON 格式：
```json
{
  "success": true,
  "broker": "simulator|ctp",
  "account_id": "...",
  "balance": 1000000.0,
  "available": 800000.0,
  "margin": 200000.0,
  "frozen_margin": 0.0,
  "commission": 150.0,
  "close_profit": 500.0,
  "position_profit": -200.0
}
```

#### Scenario: Simulator 模式资金查询
- **WHEN** counter_bridge 以 simulator 模式运行，且收到 `GET /account` 请求
- **THEN** 返回 `success: true`，`broker: "simulator"`，以及模拟账户资金数据

#### Scenario: CTP 模式资金查询
- **WHEN** counter_bridge 以 CTP 模式运行，且收到 `GET /account` 请求
- **THEN** 返回 `success: true`，`broker: "ctp"`，以及真实 CTP 账户资金数据

#### Scenario: 无可用 broker
- **WHEN** 没有任何 broker plugin 已登录，且收到 `GET /account` 请求
- **THEN** 返回 `success: false`，`error: "No broker plugin available"`

### Requirement: counter_bridge HTTP 端口为 8082

counter_bridge SHALL 在端口 8082 上启动 HTTP 服务器，避免与 OverviewServer (8080) 端口冲突。

原有 `/simulator/account` 和 `/simulator/stats` 端点 SHALL 保留，作为向后兼容。

#### Scenario: 端口监听
- **WHEN** counter_bridge 启动
- **THEN** HTTP 服务器监听 0.0.0.0:8082
