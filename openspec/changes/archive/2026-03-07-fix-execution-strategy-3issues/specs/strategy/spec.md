## MODIFIED Requirements

### Requirement: Cancel order by ID

ExecutionStrategy.sendCancelOrder(int orderID) SHALL implement CANCELREQ_PAUSE 机制：撤单被拒后，对同一 orderID 的后续撤单请求 MUST 暂停 CANCELREQ_PAUSE 纳秒后才允许重试。对不同 orderID 或无拒绝记录的撤单 MUST 立即执行。CROSS 类型订单 MUST 不可撤。

#### Scenario: Normal cancel without prior reject
- **WHEN** sendCancelOrder(orderID) 被调用，且 lastCancelReqRejectSet == 0，且订单状态为 NEW_CONFIRM
- **THEN** 订单状态变为 CANCEL_ORDER，cancelCount 递增，返回 true

#### Scenario: Cancel blocked by CANCELREQ_PAUSE
- **WHEN** sendCancelOrder(orderID) 被调用，且该 orderID 刚被拒绝（lastCancelReqRejectSet == 1），且距上次拒绝时间 < CANCELREQ_PAUSE
- **THEN** 撤单不执行，返回 false

#### Scenario: Cancel allowed after CANCELREQ_PAUSE expired
- **WHEN** sendCancelOrder(orderID) 被调用，且该 orderID 被拒绝过，但距上次拒绝时间 > CANCELREQ_PAUSE
- **THEN** 撤单正常执行，lastCancelReqRejectSet 重置为 0，返回 true

#### Scenario: Cancel different orderID not blocked
- **WHEN** sendCancelOrder(orderID_B) 被调用，且 orderID_A 刚被拒绝
- **THEN** orderID_B 的撤单正常执行，不受 orderID_A 拒绝影响

#### Scenario: CROSS order cannot be cancelled
- **WHEN** sendCancelOrder(orderID) 被调用，且订单类型为 CROSS
- **THEN** 返回 false，不执行撤单

### Requirement: Process cancel reject

ExecutionStrategy.processCancelReject() SHALL 设置 CANCELREQ_PAUSE 状态字段，清理 SelfBook CacheDel 映射，并在 fillOnCxlReject 条件下触发虚拟成交。

#### Scenario: Set cancel reject state
- **WHEN** processCancelReject 被调用
- **THEN** lastCancelReqRejectSet 设为 1，lastCancelRejectTime 设为当前时间，lastCancelRejectOrderID 设为响应中的 orderID

#### Scenario: Clean CacheDel on reject (SelfBook mode)
- **WHEN** processCancelReject 被调用，且 bSelfBook=true 且 bSnapshot=false
- **THEN** 对应方向的 bidMapCacheDel 或 askMapCacheDel 中移除该订单价格

#### Scenario: Fill on cancel reject
- **WHEN** processCancelReject 被调用，且 response.Quantity==0 且 fillOnCxlReject==true
- **THEN** 触发 processTrade 虚拟成交

### Requirement: Self-book cache delete tracking

sendCancelOrder(int) 在 bSelfBook 模式下 SHALL 将撤单中的订单插入 bidMapCacheDel/askMapCacheDel。removeOrder() SHALL 在 bSelfBook 模式下清理 bidMapCache/askMapCache。

#### Scenario: Insert into CacheDel on cancel
- **WHEN** sendCancelOrder 成功执行，且 bSelfBook=true 且 bSnapshot=false
- **THEN** 订单被插入对应方向的 bidMapCacheDel 或 askMapCacheDel，且 cxlQty 设为 openQty

#### Scenario: Clean cache on remove order (SelfBook)
- **WHEN** removeOrder 被调用，且 bSelfBook=true
- **THEN** 对应方向的 bidMapCache 或 askMapCache 中移除该订单价格

#### Scenario: Conditional ordMap removal in SelfBook mode
- **WHEN** removeOrder 被调用，且 bSelfBook=true，且订单状态为 CANCEL_CONFIRM 但 cancel=false
- **THEN** ordMap 不移除该订单，仅将 cancel 设为 true

### Requirement: HandleSquareON base class method

ExecutionStrategy SHALL 提供 handleSquareON() 基类方法，调用 sendMonitorStratStatus 上报状态。子类 MAY override 并调用 super.handleSquareON()。

#### Scenario: Base class handleSquareON
- **WHEN** ExecutionStrategy.handleSquareON() 被调用
- **THEN** 调用 sendMonitorStratStatus(product, strategyID, onExit, onCancel, onFlat, active)

#### Scenario: PairwiseArbStrategy override calls super
- **WHEN** PairwiseArbStrategy.handleSquareON() 被调用
- **THEN** 先调用 super.handleSquareON()，然后执行自身的标志重置和 avgSpreadRatio 逻辑
