# Strategy

## Requirement: 无条件 HandleSquareoff

SIGTERM 信号处理必须无条件调用 `HandleSquareoff()`，不检查 `IsActive()` 状态，对齐 C++ `main.cpp:Squareoff()` 行为。

## Requirement: Instrument OrigBaseName 字段

`Instrument` 结构体必须包含 `OrigBaseName` 字段（来自 controlFile），`SaveMatrix2` 使用 `OrigBaseName` 写入 daily_init 文件，确保与 C++ `m_origbaseName` 格式一致（如 `ag_F_3_SFE`）。

## Requirement: loadThresholds 对齐 C++ AddThreshold

ConfigParser.loadThresholds() 必须使用显式 switch-case 逐参数解析，与 C++ `TradeBotUtils::AddThreshold()` 逐分支对齐。必须包含：
- 时间单位转换：PAUSE/CONFIRM_PAUSE ×1e6（微秒→纳秒），SQROFF_TIME/STOP_LOSS_TIME 等 ×1e9（秒→纳秒）
- 副作用赋值：SIZE 同步写入 BEGIN_SIZE/BID_SIZE/ASK_SIZE，SMS_RATIO 计算 SMS_RATIO=SIZE/SMS_RATIO
- 字段重映射：DECAY→DECAY1，PRODUCT→productName，MODEL→modelName 等
- 特殊布尔处理：USE_LINEAR_THOLD 解析为 boolean
- 未知参数：抛出 IllegalArgumentException

#### Scenario: 时间单位转换
- **WHEN** 阈值文件包含 `PAUSE 500`
- **THEN** ThresholdSet.PAUSE 的值必须为 500000000.0（×1e6 微秒→纳秒）

#### Scenario: SIZE 副作用赋值
- **WHEN** 阈值文件包含 `SIZE 5`
- **THEN** ThresholdSet.SIZE、BEGIN_SIZE、BID_SIZE、ASK_SIZE 均为 5.0

#### Scenario: 字段重映射
- **WHEN** 阈值文件包含 `DECAY 0.95`
- **THEN** ThresholdSet.DECAY1 的值为 0.95（DECAY 映射到 DECAY1）

#### Scenario: 未知参数
- **WHEN** 阈值文件包含未知参数名 `UNKNOWN_PARAM 123`
- **THEN** 系统必须抛出 IllegalArgumentException

## Requirement: Tick INVALID 判断对齐 C++ FillTick

Tick 有效性检查必须使用 OR 逻辑：当 `bidQty[0]==0` **或** `askQty[0]==0` 时标记为 INVALID，对齐 C++ `Tick::FillTick()` 的 `if (bidQty[0] == 0 || askQty[0] == 0)`。

#### Scenario: 单边无量标记无效
- **WHEN** 行情 bidQty[0]=0 且 askQty[0]=10
- **THEN** Tick 状态必须为 INVALID

#### Scenario: 双边有量标记有效
- **WHEN** 行情 bidQty[0]=5 且 askQty[0]=10
- **THEN** Tick 状态必须为 VALID

## Requirement: CommonClient endPkt 处理

当 `useEndPkt=true` 且 `endPkt==1` 时，CommonClient 必须触发 INDCallBack 并立即返回，不执行后续的 CheckLastUpdate 和 SendINDUpdate。对齐 C++ `SendInfraMDUpdate()` 中的 endPkt 处理块。

#### Scenario: endPkt 触发 INDCallBack
- **WHEN** useEndPkt=true 且收到 endPkt=1 的行情包
- **THEN** 系统触发 INDCallBack 后返回，不处理后续逻辑

## Requirement: CheckLastUpdate 僵尸行情检测

CommonClient 必须实现 CheckLastUpdate()，遍历所有 simConfig 的 instruMap，检测超过 updateInterval（默认 120 秒）未收到行情的合约。检测到僵尸行情时，必须对相关策略设置 onExit=true、onCancel=true、onFlat=true。

#### Scenario: 行情超时触发退出
- **WHEN** 某合约超过 120 秒未收到行情更新
- **THEN** 该合约所属策略的 onExit、onCancel、onFlat 均设为 true

#### Scenario: 行情正常不触发
- **WHEN** 所有合约在 120 秒内有行情更新
- **THEN** 不修改任何策略的 onExit/onCancel/onFlat 状态

## Requirement: DateConfig 交易时段控制

SimConfig 必须包含 DateConfig 功能：startTimeEpoch/endTimeEpoch 字段、updateActive() 方法、initDateConfigEpoch() 初始化。simActive 默认为 false（对齐 C++ DateConfig::Reset()），由 updateActive() 根据当前时间判定。夜盘跨日场景（startTime > endTime）endTimeEpoch 必须使用下一日基准。

#### Scenario: 交易时段内
- **WHEN** 当前时间在 startTimeEpoch 和 endTimeEpoch 之间
- **THEN** updateActive() 返回 true，simActive 设为 true

#### Scenario: 夜盘跨日
- **WHEN** startTime=2100 endTime=0230（夜盘配置）
- **THEN** endTimeEpoch 基于下一日午夜计算，21:00-次日02:30 内 simActive=true

#### Scenario: 未配置时间
- **WHEN** startTime 和 endTime 均为空
- **THEN** startTimeEpoch=0，endTimeEpoch=Long.MAX_VALUE，simActive 默认为 true

## Requirement: bu tickSize 修正

ConfigParser 中 `bu`（沥青）合约的 tickSize 必须为 1.0，对齐 C++ 原代码。

#### Scenario: bu 合约 tickSize
- **WHEN** 解析 bu 合约配置
- **THEN** tickSize 为 1.0（非 2.0）

## Requirement: baseNameToSymbol 年份 rollover

baseNameToSymbol() 必须实现跨年推断逻辑，对齐 C++ `Instrument::FillChinaFields2()`：当合约月份小于当前月份时，自动使用下一年的 2 位年份。

#### Scenario: 年内合约（无 rollover）
- **WHEN** 当前月份为 3 月，baseName 为 `ag_F_6_SFE`（6 月合约）
- **THEN** 使用当前年份，返回 `ag2606`

#### Scenario: 跨年合约（需 rollover）
- **WHEN** 当前月份为 11 月，baseName 为 `ag_F_3_SFE`（3 月合约）
- **THEN** 自动使用下一年，返回 `ag2703`（非 `ag2603`）

#### Scenario: 当前月份等于合约月份
- **WHEN** 当前月份为 6 月，baseName 为 `ag_F_6_SFE`
- **THEN** 使用当前年份，返回 `ag2606`

## Requirement: fillOrderBook 完整字段填充

Instrument.fillOrderBook() 必须从 SHM MarketUpdateNew 读取以下字段，对齐 C++ `Instrument::FillOrderBook()` + `CopyOrderBook()`：
- `bidOrderCount[i]` / `askOrderCount[i]`（20 档）
- `validBids` / `validAsks`（实际有效档位数）
- `lastTradeQty`（最新成交量）
- `updateIndicators = true`（标志位）

#### Scenario: 完整字段填充
- **WHEN** 收到 MarketUpdateNew 行情包
- **THEN** bidOrderCount/askOrderCount/validBids/validAsks/lastTradeQty 均从 SHM 读取填充

#### Scenario: validBids 反映实际档位
- **WHEN** 行情只有 5 档有效买盘
- **THEN** validBids 为 5（非默认 20）

## Requirement: sendNewOrder 完整字段填充

CommonClient.sendNewOrder() 必须设置以下 C++ `CommonClient::SendNewOrder()` 中的字段：
- `Token`（合约 token）
- `QuantityFilled = 0`
- `DisclosedQnty = Quantity`
- `Product`（策略产品名）
- `AccountID`（策略账户）
- `Contract_Description.InstrumentName`
- `Contract_Description.OptionType`（CE/PE/XX）
- `Contract_Description.CALevel = 0`
- `Contract_Description.ExpiryDate`
- `Contract_Description.StrikePrice`
- `Duration`（CROSS 时为 FAK，其他为 DAY）

#### Scenario: CROSS 订单使用 FAK
- **WHEN** 发送 CROSS 类型订单
- **THEN** Duration 字段设为 FAK（Fill-and-Kill）

#### Scenario: 普通订单使用 DAY
- **WHEN** 发送普通限价订单
- **THEN** Duration 字段设为 DAY

## Requirement: m_sendMail 字段补齐

ExecutionStrategy 必须包含 `sendMail` boolean 字段，默认 false，对齐 C++ `ExecutionStrategy::m_sendMail`。

#### Scenario: 默认值
- **WHEN** 策略初始化
- **THEN** sendMail 为 false

## Requirement: SetCheckCancelQuantity 撤单数量追踪

ExecutionStrategy 必须实现 `setCheckCancelQuantity()` 方法，对齐 C++ `ExecutionStrategy::SetCheckCancelQuantity()`：当交易所为 FORTS/KRX/SFE/SGX 时 `checkCancelQuantity=true`，其他交易所为 `false`。构造函数中必须调用此方法。

#### Scenario: SFE 交易所启用撤单数量检查
- **WHEN** 合约交易所为 SFE（上海期货）
- **THEN** checkCancelQuantity 为 true

#### Scenario: 非支持交易所禁用撤单数量检查
- **WHEN** 合约交易所为 CME
- **THEN** checkCancelQuantity 为 false

## Requirement: FillMsg 请求消息构建方法

ExecutionStrategy 必须实现 `fillMsg()` 方法，对齐 C++ `ExecutionStrategy::FillMsg()`，将 sendNewOrder 中的 RequestMsg 字段设置逻辑抽取为可复用的独立方法，供 ExtraStrategy 调用。

#### Scenario: ExtraStrategy 调用 fillMsg 构建订单
- **WHEN** ExtraStrategy 需要为多合约发单
- **THEN** 可调用 fillMsg() 构建完整 RequestMsg，无需重复字段设置逻辑

## Requirement: SendInfraReqUpdate 请求队列回调

CommonClient 必须实现 `sendInfraReqUpdate()` 方法骨架，对齐 C++ `CommonClient::SendInfraReqUpdate()`。当前仅在 CommonBook 启用时执行（中国期货场景不启用），方法体为空壳 + 注释。

#### Scenario: CommonBook 未启用时不执行
- **WHEN** CommonBook 未启用（中国期货默认）
- **THEN** sendInfraReqUpdate() 立即返回，不执行任何操作

## Requirement: SimConfig currDate 进程启动日期

SimConfig 必须包含 `currDate` 字段（String, YYYYMMDD 格式），在 `initDateConfigEpoch()` 中初始化为进程启动时的日期。夜盘跨日后此字段不变（与 C++ `m_dateConfig.m_currDate` 行为一致）。

#### Scenario: 夜盘跨日后 currDate 不变
- **WHEN** 进程在 21:00 启动（currDate=20260306），运行到次日 01:00
- **THEN** currDate 仍为 "20260306"（进程启动日期）

## Requirement: lastTradePx 条件更新

Instrument.fillOrderBook() 中 lastTradePx/lastTradeQty 仅在 updateType 为 TRADE 相关类型时更新，对齐 C++ `Instrument::FillOrderBook()` 中的 `MDUPDTYPE_TRADE || MDUPDTYPE_TRADE_IMPLIED || MDUPDTYPE_TRADE_INFO` 条件。

#### Scenario: 非 TRADE 行情不更新 lastTradePx
- **WHEN** 收到 ORDER_ENTRY 类型行情更新
- **THEN** lastTradePx 保持上一次值不变

#### Scenario: TRADE 行情更新 lastTradePx
- **WHEN** 收到 TRADE 类型行情更新
- **THEN** lastTradePx 更新为 newPrice

## Requirement: Connector OrderID 溢出防护

Connector.getUniqueOrderNumber() 必须检测 orderCount 达到 ORDERID_RANGE 上限时的溢出情况，记录 SEVERE 级别日志告警。

#### Scenario: OrderID 溢出告警
- **WHEN** orderCount 达到 ORDERID_RANGE (1000000)
- **THEN** 记录 SEVERE 日志告警，返回 -1 表示失败

## Requirement: Instrument tickType 枚举

Instrument 必须包含 `tickType` 字段（枚举：BIDQUOTE/ASKQUOTE/BIDTRADE/ASKTRADE/TRADEINFO/INVALID），对齐 C++ `Tick::ticktype_t`。fillOrderBook() 后根据 updateType 设置 tickType。

#### Scenario: TRADE 行情设置 tickType
- **WHEN** 收到 TRADE 类型行情
- **THEN** tickType 设为 BIDTRADE 或 ASKTRADE（根据 side）

#### Scenario: ORDER_ENTRY 行情设置 tickType
- **WHEN** 收到 ORDER_ENTRY 类型行情
- **THEN** tickType 设为 BIDQUOTE 或 ASKQUOTE（根据 side）
