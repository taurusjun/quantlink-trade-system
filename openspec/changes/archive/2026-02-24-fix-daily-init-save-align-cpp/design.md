## Context

Go 的 `handleSquareoffLocked()` 保存 daily_init 时，将持仓拆分为 ytd 和 2day 分别保存。但 C++ `SaveMatrix2`（PairwiseArbStrategy.cpp:675-676）的语义是：关机时将全部持仓写入 ytd 字段，2day 固定为 0——下次启动时全部仓位自然变成"昨仓"，今仓从 0 开始。

此外，`main.go` shutdown 中存在第二次 daily_init 保存，与 HandleSquareoff 内部保存形成冲突。当 AVG_SPREAD_AWAY 触发 squareoff → 保存错误值 → 手动修正文件 → kill 进程 → main.go shutdown 又覆盖回错误值。

## Goals / Non-Goals

**Goals:**
- daily_init 保存字段语义与 C++ SaveMatrix2 完全一致
- 消除 main.go shutdown 中的重复保存，避免覆盖问题
- 新增测试覆盖保存字段的正确性

**Non-Goals:**
- 不修改 daily_init 文件格式或加载逻辑
- 不修改 DailyInit 结构体定义

## Decisions

**决定 1: ytd1 保存 NetposPass（total），2day 固定为 0**

C++ 原代码:
```cpp
out << m_strategyID << " " << "0 " << avgSpreadRatio_ori
    << " " << ... << " " << m_firstStrat->m_netpos_pass << " " << m_secondStrat->m_netpos_agg;
```

`m_netpos_pass` 是全部仓位（ytd + today），"0" 是固定的 2day 字段。Go 应直接使用 `NetposPass` 而非 `NetposPassYtd`。

**决定 2: 移除 main.go shutdown 中的重复保存**

C++ 只在 `HandleSquareoff()` 内部调用 `SaveMatrix2()`，没有额外的 shutdown 保存。Go 的 shutdown 流程中，如果策略 active 会先调用 `HandleSquareoff()`（其中已保存），如果策略已 inactive（如被 AVG_SPREAD_AWAY 触发过 squareoff），则内部已保存过，不需要再保存。

## Risks / Trade-offs

**[风险] 如果策略从未触发 HandleSquareoff 就被 SIGKILL** → daily_init 不会更新。但这与 C++ 行为一致，且正常 shutdown（SIGTERM/SIGINT）会先调用 HandleSquareoff。
