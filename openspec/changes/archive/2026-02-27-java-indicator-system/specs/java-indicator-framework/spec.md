## ADDED Requirements

### Requirement: Indicator 基类
系统 SHALL 提供 `Indicator` 抽象基类，保持与 C++ `TradeBot::Indicator` 完整 API 对应。

#### Scenario: Calculate 差值
- **WHEN** value=10, last_value=7, 调用 calculate()
- **THEN** diff_value=3, last_value=10

#### Scenario: isValid 为 false 时 value 归零
- **WHEN** isValid=false, 调用 calculate()
- **THEN** value=0, diff_value=-last_value

### Requirement: Dependant 指标
Dependant SHALL 作为因变量指标，从 Instrument 读取价格（MID_PX, MKTW_PX 等），支持 QuoteUpdate/TickUpdate/OrderBookStratUpdate。

#### Scenario: MID_PX 模式
- **WHEN** Instrument bidPx[0]=100, askPx[0]=102, style=MID_PX
- **THEN** QuoteUpdate 后 value=101, isValid=true

#### Scenario: bidPx 为零时无效
- **WHEN** Instrument bidPx[0]=0
- **THEN** QuoteUpdate 后 isValid=false

### Requirement: CalculateTargetPNL
CalculateTargetPNL SHALL 基于指标差值计算公允价和各档 PNL。

#### Scenario: MKTW_PX2 模式基本计算
- **WHEN** depPrice=100, 有一个非 Dep 指标 diff=0.5, coefficient=2.0, tickSize=1.0
- **THEN** targetPrice = 100 + (0.5 * 2.0 * 1.0) = 101

#### Scenario: PNL 计算
- **WHEN** targetPrice=101, bidPx=100, buyExchTx=0, sellExchTx=0, priceMultiplier=15
- **THEN** targetBidPNL[0] = (101 - 100) * 15 = 15

### Requirement: CommonClient Indicator 更新
CommonClient.sendINDUpdate() SHALL 在行情分发时调用 update() 方法更新指标值。

#### Scenario: QuoteUpdate 路径
- **WHEN** 收到 L1 行情更新 (updateLevel=1)
- **THEN** 遍历合约 indList 调用每个指标的 quoteUpdate()

### Requirement: indCallback 非 arb 路径
TraderMain indCallback SHALL 根据 useArbStrat 分流，非 arb 路径调用 CalculateTargetPNL。

#### Scenario: useArbStrat=false
- **WHEN** indCallback 触发且 useArbStrat=false
- **THEN** 调用 calculateTargetPNL → 有正 PNL 时调用 setTargetValue

#### Scenario: useArbStrat=true（现有行为不变）
- **WHEN** indCallback 触发且 useArbStrat=true
- **THEN** 直接调用 setTargetValue(0, 0, {1,...}, {1,...})
