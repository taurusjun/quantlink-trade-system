# Proposal: 修复 shutdown 保存 + origBaseName 写入

## 问题描述

### Bug 1: SIGTERM 信号下 daily_init 未保存

Go trader 在收到 SIGTERM 时，由于存在 `if pas.IsActive()` 守卫，当策略未激活时跳过 `HandleSquareoff()`，导致 daily_init 文件不保存 avgPx 等运行时状态。

**C++ 原逻辑**（`tbsrc/main/main.cpp:96-138`）：`Squareoff()` 信号处理器无条件调用 `Strategy->HandleSquareoff()`，不检查 `m_Active`。

### Bug 2: daily_init 中 origBaseName 写入错误

`SaveMatrix2` 使用 `pas.Inst1.Symbol`（如 `ag2603`）而非 `origBaseName`（如 `ag_F_3_SFE`），导致 daily_init 文件中的合约名与 C++ 格式不一致。

**C++ 原逻辑**（`tbsrc/Strategies/PairwiseArbStrategy.cpp:653-686`）：SaveMatrix2 写入 `m_instru->m_origbaseName`，该字段来自 controlFile。

## 影响范围

- `tbsrc-golang/cmd/trader/main.go` — shutdown handler + origBaseName 赋值
- `tbsrc-golang/pkg/instrument/instrument.go` — Instrument 结构体
- `tbsrc-golang/pkg/strategy/pairwise_arb.go` — SaveMatrix2 调用
- `tbsrc-golang/pkg/strategy/pairwise_arb_test.go` — 测试用例
- `data_new/common/config/config_CHINA.*.cfg` — 清理未使用字段（附带）

## 解决方案

1. 移除 `IsActive()` 守卫，对齐 C++ 无条件调用 HandleSquareoff
2. 在 Instrument 结构体中添加 `OrigBaseName` 字段
3. main.go 中从 controlFile 配置赋值 OrigBaseName
4. SaveMatrix2 使用 OrigBaseName 替代 Symbol
