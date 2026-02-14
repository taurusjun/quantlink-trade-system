# 策略引擎统计日志规范化 — 设计

## 上下文

`engine.go` 中的 `PrintStatistics()` 使用 `fmt.Println`/`fmt.Printf` 输出统计信息（9 处调用），而同文件其他 40+ 处日志均使用 `log.Printf("[StrategyEngine] ...")`。

## 目标 / 非目标

**目标：**
- 将 `PrintStatistics()` 中的 fmt 输出替换为 log.Printf，统一日志格式

**非目标：**
- 不改变统计信息的业务内容
- 不引入结构化日志库（如 zap、zerolog）——那是更大范围的改动
- 不修改 `fmt` 包的 import（其他地方仍在使用 `fmt.Errorf`/`fmt.Sprintf`）

## 决策

### 决策 1：逐行替换为 log.Printf

将每个 `fmt.Println`/`fmt.Printf` 替换为对应的 `log.Printf("[StrategyEngine] ...")`。

**映射规则：**
- `fmt.Println("text")` → `log.Printf("[StrategyEngine] text")`
- `fmt.Printf("format", args...)` → `log.Printf("[StrategyEngine] format", args...)`
- 移除 `\n` 前缀（`log.Printf` 自带换行）

### 决策 2：分隔线简化

原始代码使用 Unicode 双线字符 `════` 作为分隔线。在 `log.Printf` 输出中保留分隔线但使用 ASCII 字符 `====`，因为日志文件中 Unicode 装饰字符没有实际意义，且可能在某些日志查看工具中显示异常。
