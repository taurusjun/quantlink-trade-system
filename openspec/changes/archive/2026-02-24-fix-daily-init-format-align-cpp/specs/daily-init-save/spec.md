## ADDED Requirements

### Requirement: LoadMatrix2 解析 C++ header+data 格式

`LoadMatrix2` SHALL 解析 C++ `PairwiseArbStrategy::LoadMatrix2` (PairwiseArbStrategy.cpp:112-144) 的文件格式：第 1 行为空格分隔的 header 列名，第 2+ 行为空格分隔的数据行，首列为 strategyID。

#### Scenario: 读取 C++ 生成的 daily_init 文件

- **WHEN** 文件内容为：
  ```
  StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
  92201 0 96.671581 ag2603 ag2605 83 -83
  ```
  且请求 strategyID=92201
- **THEN** 返回 DailyInit{StrategyID:92201, Netpos2day1:0, AvgSpreadOri:96.671581, OrigBaseName1:"ag2603", OrigBaseName2:"ag2605", NetposYtd1:83, NetposAgg2:-83}

#### Scenario: strategyID 不匹配时返回错误

- **WHEN** 文件中不包含请求的 strategyID 对应的数据行
- **THEN** 返回错误

### Requirement: SaveMatrix2 输出 C++ header+data 格式

`SaveMatrix2` SHALL 输出与 C++ `PairwiseArbStrategy::SaveMatrix2` (PairwiseArbStrategy.cpp:653-686) 完全一致的格式。

#### Scenario: 保存的文件可被 C++ LoadMatrix2 读取

- **WHEN** 保存 DailyInit{StrategyID:92201, AvgSpreadOri:96.671581, OrigBaseName1:"ag2603", OrigBaseName2:"ag2605", NetposYtd1:83, NetposAgg2:-83}
- **THEN** 文件内容为：
  ```
  StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
  92201 0 96.671581 ag2603 ag2605 83 -83
  ```
- **AND** header 列顺序与 C++ `SaveMatrix2` 完全一致
- **AND** avgPx 使用 fixed-point 格式（`%.6f` 或 C++ `ios::fixed` 默认精度）

### Requirement: 方法名与 C++ 一致

Go 函数名 SHALL 使用 `LoadMatrix2` / `SaveMatrix2`，与 C++ `PairwiseArbStrategy::LoadMatrix2` / `PairwiseArbStrategy::SaveMatrix2` 保持一致。

#### Scenario: 旧方法名不再存在

- **WHEN** 编译 Go 代码
- **THEN** `LoadDailyInit` 和 `SaveDailyInit` 函数不存在
- **AND** 所有调用方使用 `LoadMatrix2` 和 `SaveMatrix2`
