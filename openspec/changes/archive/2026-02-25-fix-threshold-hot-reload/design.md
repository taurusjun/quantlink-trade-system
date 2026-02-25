## Context

当前热加载流程：
```
main goroutine:  config.Load() → thold1.LoadFromMap() → thold2.LoadFromMap()
pollMD goroutine: pas.mu.Lock() → pas.Thold1.BeginPlace (读取) → pas.mu.Unlock()
```
两个 goroutine 并发访问同一 `*ThresholdSet`，无同步。

此外 `SpreadTracker.Alpha`、`SpreadTracker.AvgSpreadAway`、`pas.MaxQuoteLevel` 在构造时从 `thold1` 复制，后续 reload `thold1` 不会同步到这些副本。

## Goals / Non-Goals

**Goals:**
- reload 操作持有 `pas.mu`，消除数据竞争
- reload 后同步更新 `SpreadTracker.Alpha`、`SpreadTracker.AvgSpreadAway`、`pas.MaxQuoteLevel`
- reload 日志打印关键参数变化，便于运维确认

**Non-Goals:**
- 不改变 reload 触发方式（仍保留 SIGUSR2 + REST API 双路径）
- 不增加配置文件格式验证
- 不增加文件监听（fsnotify）自动 reload

## Decisions

### Decision 1: 将 reload 逻辑封装到 PairwiseArbStrategy.ReloadThresholds()

将 `LoadFromMap` + 副本同步 + 日志打印封装为策略方法，在 `pas.mu` 保护下执行。main.go 只负责读取配置文件，把 map 传给策略。

理由：
- 策略拥有 mutex 和所有需要同步的字段
- 与 `HandleSquareoff()`、`HandleSquareON()` 等方法模式一致（外部调用 → 加锁 → 内部执行）
- main.go 保持简洁

### Decision 2: SpreadTracker 直接更新字段，不重建

`SpreadTracker` 的 `Alpha` 和 `AvgSpreadAway` 是简单字段，直接赋值即可。不需要重建整个 `SpreadTracker`，否则会丢失 `AvgSpreadOri`（EWA 累积值）。

### Decision 3: 日志打印 before/after 对比

reload 日志同时打印修改前和修改后的值，方便运维确认变化。只打印关键参数：`BeginPlace`、`LongPlace`、`ShortPlace`、`Size`、`MaxSize`、`Alpha`。
