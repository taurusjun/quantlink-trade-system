## MODIFIED Requirements

### Requirement: ③ Account Table（右侧）

列: Broker | AccountID | TotalAsset | AvailCash | Margin | Risk(%) | ClosePnL | PosPnL

OverviewServer SHALL 每 10 秒通过 HTTP GET 查询 `http://localhost:8082/account` 获取资金数据。

查询结果 SHALL 缓存为 `AccountRow` 列表，在每次聚合 OverviewSnapshot 时合并到 `accounts` 字段。

Account Table SHALL 通过 Vue 数据绑定渲染 `overview.accounts` 数组。

Risk(%) SHALL 计算为 `margin / balance * 100`，超过 50% 时以红色高亮显示。

当 counter_bridge 未启动或查询失败时，Account Table SHALL 显示 "Waiting for counter_bridge..." 占位文字。

#### Scenario: 资金数据正常展示
- **WHEN** counter_bridge 运行中，且 OverviewServer 成功查询到资金数据
- **THEN** Account Table 显示 broker 名称、账户 ID、总资产、可用资金、保证金、风险度、平仓盈亏、持仓盈亏

#### Scenario: counter_bridge 未启动
- **WHEN** counter_bridge 未运行，OverviewServer 查询 `localhost:8082/account` 连接失败
- **THEN** Account Table 显示 "Waiting for counter_bridge..."，不影响其他区域正常展示

#### Scenario: 资金数据定时刷新
- **WHEN** counter_bridge 运行中
- **THEN** Account Table 数据每 10 秒自动刷新一次
