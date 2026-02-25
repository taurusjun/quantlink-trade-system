## 1. PairwiseArbStrategy 新增 ReloadThresholds 方法

- [x] 1.1 在 `pairwise_arb.go` 新增 `ReloadThresholds(firstMap, secondMap map[string]float64)` 方法：持有 `pas.mu`，记录旧值，调用 `LoadFromMap`，同步 `Spread.Alpha`/`Spread.AvgSpreadAway`/`MaxQuoteLevel`，打印 before/after 日志

## 2. 更新 main.go 的 reloadThresholds 闭包

- [x] 2.1 修改 `cmd/trader/main.go` 中的 `reloadThresholds` 闭包：`config.Load()` 后调用 `pas.ReloadThresholds(firstMap, secondMap)` 替代直接 `thold.LoadFromMap()`

## 3. 测试

- [x] 3.1 新增 `pairwise_arb_test.go` 中的 `TestReloadThresholds` 单元测试，验证 reload 后 ThresholdSet 字段、SpreadTracker.Alpha、MaxQuoteLevel 均已更新
- [x] 3.2 运行 `go test -race ./pkg/strategy/...` 确认无数据竞争
