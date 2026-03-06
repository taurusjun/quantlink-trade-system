## MODIFIED Requirements

### Requirement: 无条件 HandleSquareoff

SIGTERM 信号处理必须无条件调用 `HandleSquareoff()`，不检查 `IsActive()` 状态，对齐 C++ `main.cpp:Squareoff()` 行为。

（此需求未变更，保持原样。）

### Requirement: Instrument OrigBaseName 字段

`Instrument` 结构体必须包含 `OrigBaseName` 字段（来自 controlFile），`SaveMatrix2` 使用 `OrigBaseName` 写入 daily_init 文件，确保与 C++ `m_origbaseName` 格式一致（如 `ag_F_3_SFE`）。

（此需求未变更，保持原样。）

## ADDED Requirements

### Requirement: loadThresholds 对齐 C++ AddThreshold

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

### Requirement: Tick INVALID 判断对齐 C++ FillTick

Tick 有效性检查必须使用 OR 逻辑：当 `bidQty[0]==0` **或** `askQty[0]==0` 时标记为 INVALID，对齐 C++ `Tick::FillTick()` 的 `if (bidQty[0] == 0 || askQty[0] == 0)`。

#### Scenario: 单边无量标记无效
- **WHEN** 行情 bidQty[0]=0 且 askQty[0]=10
- **THEN** Tick 状态必须为 INVALID

#### Scenario: 双边有量标记有效
- **WHEN** 行情 bidQty[0]=5 且 askQty[0]=10
- **THEN** Tick 状态必须为 VALID

### Requirement: CommonClient endPkt 处理

当 `useEndPkt=true` 且 `endPkt==1` 时，CommonClient 必须触发 INDCallBack 并立即返回，不执行后续的 CheckLastUpdate 和 SendINDUpdate。对齐 C++ `SendInfraMDUpdate()` 中的 endPkt 处理块。

#### Scenario: endPkt 触发 INDCallBack
- **WHEN** useEndPkt=true 且收到 endPkt=1 的行情包
- **THEN** 系统触发 INDCallBack 后返回，不处理后续逻辑

### Requirement: CheckLastUpdate 僵尸行情检测

CommonClient 必须实现 CheckLastUpdate()，遍历所有 simConfig 的 instruMap，检测超过 updateInterval（默认 120 秒）未收到行情的合约。检测到僵尸行情时，必须对相关策略设置 onExit=true、onCancel=true、onFlat=true。

#### Scenario: 行情超时触发退出
- **WHEN** 某合约超过 120 秒未收到行情更新
- **THEN** 该合约所属策略的 onExit、onCancel、onFlat 均设为 true

#### Scenario: 行情正常不触发
- **WHEN** 所有合约在 120 秒内有行情更新
- **THEN** 不修改任何策略的 onExit/onCancel/onFlat 状态

### Requirement: DateConfig 交易时段控制

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

### Requirement: bu tickSize 修正

ConfigParser 中 `bu`（沥青）合约的 tickSize 必须为 1.0，对齐 C++ 原代码。

#### Scenario: bu 合约 tickSize
- **WHEN** 解析 bu 合约配置
- **THEN** tickSize 为 1.0（非 2.0）
