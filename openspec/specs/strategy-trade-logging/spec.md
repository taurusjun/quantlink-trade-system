## ADDED Requirements

### Requirement: 发单日志输出
ExecutionStrategy.sendNewOrder() 在成功发送订单后 SHALL 输出 log.info，包含：symbol、side（BUY/SELL）、price、quantity、orderID。

#### Scenario: 普通限价单发送
- **WHEN** sendNewOrder() 成功将订单写入 SHM
- **THEN** 输出 log.info 格式: `[ORDER-NEW] symbol=ag2603 side=BUY price=24577 qty=1 orderID=16000001`

#### Scenario: 撤单请求发送
- **WHEN** sendCancelOrder() 发送撤单请求
- **THEN** 输出 log.info 格式: `[ORDER-CANCEL] orderID=16000001 symbol=ag2603`

### Requirement: 成交回报日志输出
ExecutionStrategy.processTrade() 在收到成交回报后 SHALL 输出 log.info，包含：symbol、side、price、qty、orderID、cumQty、remainQty。

#### Scenario: 首笔成交
- **WHEN** orsCallBack 收到 TRADE_CONFIRM 且订单 cumQty 从 0 变为 1
- **THEN** 输出 log.info 格式: `[TRADE] symbol=ag2603 side=BUY price=24577 qty=1 orderID=16000001 cumQty=1 remainQty=0`

#### Scenario: 部分成交
- **WHEN** orsCallBack 收到 TRADE_CONFIRM 且订单 remainQty > 0
- **THEN** 输出 log.info 格式包含 cumQty 和 remainQty 以区分部分/完全成交

### Requirement: 撤单/拒绝回报日志输出
ExecutionStrategy 在收到撤单确认或新单拒绝后 SHALL 输出日志。

#### Scenario: 撤单确认
- **WHEN** orsCallBack 收到 CANCEL_ORDER_CONFIRM
- **THEN** 输出 log.info 格式: `[CANCEL-CONFIRM] orderID=16000001 symbol=ag2603`

#### Scenario: 新单拒绝
- **WHEN** orsCallBack 收到 NEW_ORDER_REJECT
- **THEN** 输出 log.warning 格式: `[ORDER-REJECT] orderID=16000001 symbol=ag2603`

#### Scenario: 撤单拒绝
- **WHEN** orsCallBack 收到 CANCEL_ORDER_REJECT
- **THEN** 输出 log.warning 格式: `[CANCEL-REJECT] orderID=16000001 symbol=ag2603`

### Requirement: 策略状态变化日志
ExecutionStrategy 在关键状态标志变化时 SHALL 输出 log.info。

#### Scenario: onFlat 状态变化
- **WHEN** checkSquareoff() 中 onFlat 从 false 变为 true
- **THEN** 输出 log.info 格式: `[STATE] onFlat: false -> true, reason=<触发原因>`

#### Scenario: onExit 状态变化
- **WHEN** checkSquareoff() 中 onExit 从 false 变为 true
- **THEN** 输出 log.info 格式: `[STATE] onExit: false -> true, reason=<触发原因>`

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
