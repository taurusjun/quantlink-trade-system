## ADDED Requirements

### Requirement: 线程安全的阈值热加载

`PairwiseArbStrategy` 提供 `ReloadThresholds()` 方法，在持有 `pas.mu` 的情况下更新所有阈值及其副本。

#### Scenario: reload 操作持有 mutex

- **WHEN** 调用 `pas.ReloadThresholds(firstMap, secondMap)`
- **THEN** 在 `pas.mu.Lock()` 保护下执行 `thold1.LoadFromMap(firstMap)` 和 `thold2.LoadFromMap(secondMap)`
- **AND** `pollMD` goroutine 的 `MDCallBack` 不会读到半更新的 ThresholdSet

#### Scenario: reload 同步过期参数

- **WHEN** reload 后 `thold1.Alpha` 发生变化
- **THEN** `pas.Spread.Alpha` 同步更新为新值
- **AND** `pas.Spread.AvgSpreadAway` 同步更新为 `thold1.AvgSpreadAway`
- **AND** `pas.MaxQuoteLevel` 同步更新为 `thold1.MaxQuoteLevel`（若 > 0）

#### Scenario: reload 日志打印 before/after

- **WHEN** reload 完成
- **THEN** 日志包含修改前和修改后的关键参数值
- **AND** 至少包含 `BeginPlace`、`LongPlace`、`ShortPlace`、`Size`、`MaxSize`、`Alpha`

#### Scenario: main.go 调用路径

- **WHEN** 收到 SIGUSR2 信号或 REST API `reload-thresholds` 请求
- **THEN** main.go 调用 `config.Load()` 读取 YAML
- **AND** 调用 `pas.ReloadThresholds(firstMap, secondMap)` 而非直接调用 `LoadFromMap`

#### Scenario: go test -race 通过

- **WHEN** 并发执行 reload 和 MDCallBack
- **THEN** `go test -race` 不报告数据竞争
