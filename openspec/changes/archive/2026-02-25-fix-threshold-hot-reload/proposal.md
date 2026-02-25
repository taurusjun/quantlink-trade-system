## Why

阈值热加载（SIGUSR2 / REST API `reload-thresholds`）存在三个问题，导致无法安全、完整地在运行时更新策略参数：

1. **数据竞争**: `reloadThresholds()` 在 main goroutine 中直接调用 `thold1.LoadFromMap()`，不持有 `pas.mu`，而 `pollMD` goroutine 的 `MDCallBack` 持有 `pas.mu` 时读取同一 `ThresholdSet` 字段 → 可能读到半更新状态
2. **过期参数**: `alpha`、`max_quote_level`、`avg_spread_away` 在构造时复制到 `SpreadTracker` 和 `pas.MaxQuoteLevel`，reload 后不同步更新 → 这些参数的修改永远不生效
3. **reload 确认缺失**: 日志仅输出 `"阈值已热加载"`，不打印新值 → 运维无法确认参数是否正确更新

## What Changes

- `PairwiseArbStrategy` 新增 `ReloadThresholds()` 方法，持有 `pas.mu` 后调用 `LoadFromMap()`，并同步更新 `SpreadTracker.Alpha`/`SpreadTracker.AvgSpreadAway` 和 `pas.MaxQuoteLevel`
- `cmd/trader/main.go` 的 `reloadThresholds` 闭包改为调用 `pas.ReloadThresholds()`
- reload 后日志打印关键参数新值

## Capabilities

### New Capabilities
- `safe-threshold-reload`: 线程安全的阈值热加载，覆盖 mutex 保护、过期参数同步、reload 日志

### Modified Capabilities

## Impact

- `tbsrc-golang/pkg/strategy/pairwise_arb.go` — 新增 `ReloadThresholds()` 方法
- `tbsrc-golang/cmd/trader/main.go` — `reloadThresholds` 闭包改为调用策略方法
- `tbsrc-golang/pkg/strategy/spread_tracker.go` — 可能需要暴露 Alpha/AvgSpreadAway 的更新方法
