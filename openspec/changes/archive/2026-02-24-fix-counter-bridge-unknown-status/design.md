## Context

CTP 对每笔订单触发多次 `OnRtnOrder` 回调，状态依次为：

```
OnRtnOrder(Unknown)        → 已提交，尚未到交易所
OnRtnOrder(NoTradeQueueing) → 交易所已确认排队
OnRtnOrder(AllTraded)       → 全部成交
OnRtnTrade                  → 成交明细
```

C++ ORS (China/ORSServer.cpp:2222-2378) 使用 `OrderSubmitStatus` + `OrderStatus` 双层 switch，对 `InsertSubmitted + Unknown` 只记录日志，不推送 response。

counter_bridge 的 `ConvertOrder` 只看 `OrderStatus`，将 `Unknown` 映射为 `OrderStatus::UNKNOWN`，在 `OnBrokerOrderCallback` 的 switch default 分支变成 `ORDER_ERROR`。Go 策略层收到假 reject 后执行 RemoveOrder，导致后续真实 CONFIRM/TRADE 找不到订单，丢失成交。

## Goals / Non-Goals

**Goals:**
- counter_bridge 对齐 C++ ORS 的 CTP 回调过滤语义，不转发中间状态
- Go 策略层 processNewReject 恢复 C++ 原始行为（立即 RemoveOrder）
- 回退之前在 Go 层做的 workaround 代码

**Non-Goals:**
- 不改变 CTP 插件的 ConvertOrder 逻辑（仅在 counter_bridge 回调层过滤）
- 不引入 counter_bridge 层的订单状态跟踪（避免增加复杂度）

## Decisions

### Decision 1: 在 counter_bridge 回调入口过滤，不改 CTP 插件

在 `OnBrokerOrderCallback` 函数开头对 UNKNOWN/SUBMITTING 状态 early return。

**理由**: 这是最小改动点。CTP 插件是通用插件层，不应硬编码业务过滤逻辑。counter_bridge 作为 ORS 角色承担过滤职责，与 C++ ORS 定位一致。

**替代方案**: 在 CTP 插件 `OnRtnOrder` 中不触发 UNKNOWN 状态的 callback → 但这会影响其他可能使用该插件的组件，不够通用。

### Decision 2: 回退 Go 侧 workaround，恢复 C++ 原始 processNewReject

之前为适应 counter_bridge 的错误行为，在 Go 层做了：
- processNewReject 不调用 RemoveOrder，保留在 OrdMap
- 添加 restoreAfterReject 处理迟到回报
- 添加 CleanupRejectedOrders 清理残留

修复 counter_bridge 后，这些 workaround 应全部回退，恢复与 C++ 一致的简洁逻辑。

**理由**: 保持 Go 策略层与 C++ ExecutionStrategy 的一致性，减少分歧点，降低维护负担。

## Risks / Trade-offs

- [风险] 如果 CTP 在真正 reject (OnRspOrderInsert) 后仍发送 OnRtnOrder → counter_bridge 已有的 `g_order_map` 查找会找不到（因为 SendOrder 返回空 broker_order_id 时不会插入 g_order_map），所以不会产生回报，安全。
- [风险] 回退 Go workaround 后，如果还有其他路径产生假 reject → 增加 counter_bridge 日志，记录所有被过滤的 UNKNOWN/SUBMITTING 回调，便于排查。
