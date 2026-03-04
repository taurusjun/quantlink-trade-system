## ADDED Requirements

### Requirement: ConvertPosition 昨仓量使用实际剩余值
`ConvertPosition()` SHALL 计算 `yesterday_volume` 为 `Position - TodayPosition`，而非直接使用 CTP 的 `YdPosition` 字段。

#### Scenario: SHFE 合约有今仓和昨仓
- **WHEN** CTP 返回 SHFE 今仓记录 `Position=5, TodayPosition=5, YdPosition=0` 和昨仓记录 `Position=1, TodayPosition=0, YdPosition=8`
- **THEN** 今仓记录的 `yesterday_volume` SHALL 为 0，昨仓记录的 `yesterday_volume` SHALL 为 1

#### Scenario: SHFE 合约昨仓已全部平完
- **WHEN** CTP 返回昨仓记录 `Position=0, TodayPosition=0, YdPosition=6`
- **THEN** `yesterday_volume` SHALL 为 0（不是 6）

#### Scenario: 非 SHFE 交易所不区分今昨
- **WHEN** CTP 返回持仓记录 `Position=10, TodayPosition=0, YdPosition=0`
- **THEN** `yesterday_volume` SHALL 为 10

### Requirement: g_mapContractPos 初始化数据正确
counter_bridge 的 `g_mapContractPos` 初始化 SHALL 反映实际持仓，`ONPos + todayPos` 之和 SHALL 等于 CTP 报告的总持仓量。

#### Scenario: 初始化后持仓总量一致
- **WHEN** ag2606 实际 Short 持仓为 6 手（今5+昨1）
- **THEN** `g_mapContractPos["ag2606"].ONShortPos + todayShortPos` SHALL 等于 6

### Requirement: yesterday_volume 防护非负
`yesterday_volume` SHALL 不小于 0。当 `Position < TodayPosition`（异常情况）时，SHALL 取 0。

#### Scenario: Position 小于 TodayPosition
- **WHEN** CTP 返回 `Position=3, TodayPosition=5`（异常数据）
- **THEN** `yesterday_volume` SHALL 为 0
