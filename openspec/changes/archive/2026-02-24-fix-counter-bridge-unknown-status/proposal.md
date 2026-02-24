## Why

counter_bridge 将 CTP `OnRtnOrder` 回调中 `OrderStatus=Unknown`（已提交待确认）的正常中间状态错误映射为 `ORDER_ERROR`，导致 Go 策略层收到假 reject，随后的真实 CONFIRM/TRADE 回报因订单已被删除而丢失，造成实盘持仓不一致。

C++ ORS (China/ORSServer.cpp) 使用 `OrderSubmitStatus` + `OrderStatus` 双层 switch 正确处理此状态，counter_bridge 缺少这一层判断。

## What Changes

- counter_bridge `OnBrokerOrderCallback` 对 `UNKNOWN` 和 `SUBMITTING` 状态不再生成 response，直接 return，与 C++ ORS 行为一致
- CTP 插件 `ConvertOrder` 补充对 `OrderSubmitStatus=InsertRejected` 的检查，确保真正的拒绝被正确标记
- 同时回退 Go 策略层之前为此问题做的 workaround（processNewReject 保留 OrdMap 等）

## Capabilities

### New Capabilities
- `bridge-order-status-filter`: counter_bridge 对 CTP 订单状态的正确过滤，对齐 C++ ORS 双层 switch 语义

### Modified Capabilities

## Impact

- `gateway/src/counter_bridge.cpp`: OnBrokerOrderCallback 增加 UNKNOWN/SUBMITTING 过滤
- `gateway/plugins/ctp/src/ctp_td_plugin.cpp`: ConvertOrder 增加 OrderSubmitStatus 检查
- `tbsrc-golang/pkg/execution/ors_callback.go`: 回退 processNewReject workaround
- `tbsrc-golang/pkg/execution/order_manager.go`: 回退 CleanupRejectedOrders
