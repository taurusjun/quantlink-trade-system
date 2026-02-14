# 策略引擎统计日志规范化 — 任务清单

## 1. 替换日志调用

- [x] 1.1 将 `PrintStatistics()` 中 3 个 `fmt.Println` 调用（分隔线和标题）替换为 `log.Printf("[StrategyEngine] ...")`，分隔线改用 ASCII `====`
- [x] 1.2 将循环体内 5 个 `fmt.Printf` 调用（策略详情）替换为 `log.Printf("[StrategyEngine] ...")`，移除多余的 `\n` 前缀

## 2. 验证

- [x] 2.1 确认 `go build` 编译通过
- [x] 2.2 确认 `go vet ./pkg/strategy/` 无新增警告
