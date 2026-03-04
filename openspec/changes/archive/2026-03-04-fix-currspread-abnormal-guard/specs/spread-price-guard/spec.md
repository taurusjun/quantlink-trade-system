## ADDED Requirements

### Requirement: Spread 计算前行情有效性守卫
currSpreadRatio 计算 SHALL 仅在两腿四个价格（firstBid, firstAsk, secondBid, secondAsk）全部 > 0 时执行。当任一价格 <= 0 时，currSpreadRatio MUST 保持上一次的有效值不变。

#### Scenario: 两腿行情完整
- **WHEN** firstBid > 0 AND firstAsk > 0 AND secondBid > 0 AND secondAsk > 0
- **THEN** 正常计算 currSpreadRatio = midPx(leg1) - midPx(leg2)

#### Scenario: Second leg bidPx 为 0
- **WHEN** secondBid = 0 AND secondAsk > 0
- **THEN** 跳过 spread 计算，currSpreadRatio 保持上一次值

#### Scenario: First leg askPx 为 0
- **WHEN** firstAsk = 0
- **THEN** 跳过 spread 计算，currSpreadRatio 保持上一次值

#### Scenario: 两腿均正常后恢复计算
- **WHEN** 之前因某腿价格为 0 跳过计算，随后两腿四个价格恢复为 > 0
- **THEN** 恢复正常计算 currSpreadRatio

### Requirement: C++ 差异注释
守卫条件的变更 MUST 在代码中加 `[C++差异]` 注释，说明与 C++ 原代码的区别及修改原因。
Ref: PairwiseArbStrategy.cpp:496

#### Scenario: 注释标注
- **WHEN** 修改守卫条件
- **THEN** 注释中包含 C++ 原代码、差异说明、修改原因
