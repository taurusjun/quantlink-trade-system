# Strategy: Shutdown 保存 + OrigBaseName

## Requirement: 无条件 HandleSquareoff

SIGTERM 信号处理必须无条件调用 `HandleSquareoff()`，不检查 `IsActive()` 状态，对齐 C++ `main.cpp:Squareoff()` 行为。

## Requirement: Instrument OrigBaseName 字段

`Instrument` 结构体必须包含 `OrigBaseName` 字段（来自 controlFile），`SaveMatrix2` 使用 `OrigBaseName` 写入 daily_init 文件，确保与 C++ `m_origbaseName` 格式一致（如 `ag_F_3_SFE`）。
