## MODIFIED Requirements

### Requirement: 策略状态变化日志
ExecutionStrategy 在关键状态标志变化时 SHALL 输出 log.info，**并同时调用 alertCollector.add() 记录告警事件**。

#### Scenario: onFlat 状态变化
- **WHEN** checkSquareoff() 中 onFlat 从 false 变为 true
- **THEN** 输出 log.info 格式: `[STATE] onFlat: false -> true, reason=<触发原因>`
- **THEN** 调用 alertCollector.add() 记录对应类型的 AlertEvent

#### Scenario: onExit 状态变化
- **WHEN** checkSquareoff() 中 onExit 从 false 变为 true
- **THEN** 输出 log.info 格式: `[STATE] onExit: false -> true, reason=<触发原因>`
- **THEN** 调用 alertCollector.add() 记录对应类型的 AlertEvent

#### Scenario: active 状态变化
- **WHEN** active 从 true 变为 false（或反向）
- **THEN** 输出 log.info 格式: `[STATE] active: true -> false, reason=<触发原因>`

### Requirement: PairwiseArbStrategy 配对级日志
PairwiseArbStrategy 在配对级别操作时 SHALL 输出额外日志。

#### Scenario: 追单发送
- **WHEN** sendAggressiveOrder() 发送追单
- **THEN** 输出 log.info 格式: `[AGG-ORDER] leg=firstStrat/secondStrat side=BUY price=24577 qty=1 aggRepeat=2`

#### Scenario: 配对成交回报路由
- **WHEN** PairwiseArbStrategy.orsCallBack() 将回报路由到 firstStrat 或 secondStrat
- **THEN** 输出 log.info 格式: `[PAIR-ORS] orderID=16000001 routed=firstStrat type=TRADE_CONFIRM`

#### Scenario: 策略停用
- **WHEN** PairwiseArbStrategy.handleSquareoff() 设置 active=false 并保存 daily_init
- **THEN** 输出 log.warning 格式: `[PAIR-EXIT] active=false avgSpread=524.96 ytd1=69 ytd2=-69`

#### Scenario: AVG_SPREAD_AWAY 触发
- **WHEN** PairwiseArbStrategy 检测到价差偏离超限
- **THEN** 输出 log.warning 并调用 alertCollector.add() 记录 AVG_SPREAD_AWAY 告警事件
