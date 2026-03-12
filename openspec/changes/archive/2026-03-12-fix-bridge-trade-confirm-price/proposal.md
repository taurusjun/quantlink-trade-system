## Why

`counter_bridge.cpp` 的 `OnBrokerOrderCallback` 在生成 TRADE_CONFIRM 回报时使用了订单限价（`LimitPrice`）和累计成交量（`traded_volume`），而非 CTP 实际成交价和单笔成交量。这导致策略层收到的成交价格失真，PnL 计算和持仓均价全部错误。复盘发现此 bug 为 Critical 级别，需立即修复。

## What Changes

- `OnBrokerOrderCallback` 不再为 PARTIAL_FILLED/FILLED 状态生成 TRADE_CONFIRM，仅保留 NEW_ORDER_CONFIRM 和 CANCEL_ORDER_CONFIRM
- `OnBrokerTradeCallback` 重写为完整的 TRADE_CONFIRM 生成器，使用 `TradeInfo.price`（实际成交价）和 `TradeInfo.volume`（单笔成交量）
- 新增 `g_order_sys_id_map` 反向映射（OrderSysID → broker_order_id），解决 CTP 模式下 TradeInfo 与 CachedOrderInfo 的关联问题
- Simulator 插件修改 `ConvertToOrderInfo` 将 `order_id` 写入 `client_order_id`（模拟 CTP 的 OrderSysID），与 CTP 行为保持一致

## Capabilities

### New Capabilities
- `bridge-trade-confirm`: Counter Bridge 成交回报生成逻辑 — 由 OnBrokerTradeCallback 统一生成 TRADE_CONFIRM，使用实际成交价和单笔成交量

### Modified Capabilities
- `bridge-order-status-filter`: OnBrokerOrderCallback 移除 TRADE_CONFIRM 生成，FILLED/PARTIAL_FILLED 状态不再发送回报

## Impact

- 修改文件: `gateway/src/counter_bridge.cpp`、`gateway/plugins/simulator/src/simulator_plugin.cpp`（2 个文件）
- 影响: 所有通过 counter_bridge 的成交回报（CTP 模式和 Simulator 模式）
- 下游: 策略层的 orsCallBack 收到的 Price 和 Quantity 字段语义变化（从限价/累计量 → 成交价/单笔量）
- 以 CTP 实盘为基准，Simulator 适配与 CTP 一致，不在 counter_bridge 中做 Simulator 特殊逻辑
