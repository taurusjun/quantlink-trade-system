## ADDED Requirements

### Requirement: counter_bridge 过滤 CTP 中间状态回调

counter_bridge 的 `OnBrokerOrderCallback` 对 CTP 订单中间状态（UNKNOWN、SUBMITTING）不得生成 response 推送给策略层，与 C++ ORS (China/ORSServer.cpp:2263-2268) 行为一致。

#### Scenario: CTP 返回 OrderStatus=Unknown 时不推送 response

- **WHEN** CTP 插件触发 `OnBrokerOrderCallback`，且 `order_info.status == UNKNOWN`
- **THEN** counter_bridge 不写入任何 ResponseMsg 到 response SHM queue，直接 return

#### Scenario: CTP 返回 OrderStatus=Submitting 时不推送 response

- **WHEN** CTP 插件触发 `OnBrokerOrderCallback`，且 `order_info.status == SUBMITTING`
- **THEN** counter_bridge 不写入任何 ResponseMsg 到 response SHM queue，直接 return

#### Scenario: 正常的 reject/confirm/trade 不受影响

- **WHEN** CTP 插件触发 `OnBrokerOrderCallback`，且 `order_info.status` 为 ACCEPTED、FILLED、PARTIAL_FILLED、CANCELED、REJECTED 或 ERROR
- **THEN** counter_bridge 按原有逻辑正常生成 ResponseMsg 并写入 response SHM queue

### Requirement: Go 策略层 processNewReject 恢复 C++ 原始语义

Go 侧 `processNewReject` 须恢复为与 C++ `ExecutionStrategy::ProcessNewReject` 一致的行为：设置状态为 `StatusNewReject` 后立即调用 `RemoveOrder` 从所有 map 中完全删除订单。

#### Scenario: processNewReject 立即删除订单

- **WHEN** Go 策略收到 ORDER_ERROR 类型的 response
- **THEN** `processNewReject` 须将订单从 OrdMap、BidMap/AskMap、Client.orderIDMap 中全部删除
- **AND** 与 C++ ProcessNewReject (ExecutionStrategy.cpp:1803-1829) + RemoveOrder (ExecutionStrategy.cpp:1175-1213) 行为一致

#### Scenario: 不再保留 rejected 订单等待迟到回报

- **WHEN** processNewReject 执行完成后
- **THEN** OrdMap 中不存在该 orderID 的条目
- **AND** CleanupRejectedOrders 方法被移除（不再需要）
