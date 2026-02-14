# 移除 CTP 行情调试打印 — 任务清单

## 1. 移除调试代码

- [x] 1.1 删除 `ctp_md_plugin.cpp` 中第 315-323 行的 ag2603 调试打印代码块（含注释）

## 2. 验证

- [x] 2.1 确认删除后代码编译通过（`ctp_md_plugin` 目标编译成功）
- [x] 2.2 确认 `OnRtnDepthMarketData()` 的正常处理流程不受影响（代码审查确认：receive_time → ConvertMarketData → Push 流程完整）
