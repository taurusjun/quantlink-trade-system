# 规范化策略引擎统计输出日志

## 为什么要做这个改动

`StrategyEngine.PrintStatistics()` 方法使用 9 个 `fmt.Println`/`fmt.Printf` 调用直接输出到 stdout，而同文件中所有其他日志均使用 `log.Printf("[StrategyEngine] ...")` 格式。这导致：

- 统计输出缺少时间戳和日志级别前缀
- 无法通过日志采集工具统一管理
- 与代码库中的日志规范不一致

## 改动内容

- 将 `PrintStatistics()` 中的 `fmt.Println`/`fmt.Printf` 替换为 `log.Printf`，使用 `[StrategyEngine]` 前缀
- 保留原有的统计信息内容和格式化方式，仅改变输出通道

## 能力变更

### 修改的能力
- `engine-statistics-logging`：统计输出从 stdout 改为标准日志通道，带时间戳和模块前缀

## 影响范围

- `golang/pkg/strategy/engine.go`：修改 `PrintStatistics()` 方法（第 659-683 行），将 9 个 fmt 调用替换为 log.Printf
