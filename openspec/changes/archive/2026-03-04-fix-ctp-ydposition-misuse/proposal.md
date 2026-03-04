## Why

CTP 的 `InvestorPositionField.YdPosition` 字段含义是"昨日结算时持仓量"（固定不变），而非"当前剩余昨仓量"。`ConvertPosition()` 将 `YdPosition` 直接赋给 `yesterday_volume`，导致 `g_mapContractPos` 中的 `ONPos`（昨仓）被严重高估。`SetCombOffsetFlag()` 据此发出错误的平昨仓指令，CTP 返回 ErrorID 51（平仓量超过持仓量）。

实盘 2026-03-04 日盘出现 3 次 ORDER-REJECT，均为此原因。

## What Changes

- 修复 `ConvertPosition()` 中 `yesterday_volume` 的计算：`YdPosition` → `Position - TodayPosition`
- 确保 `g_mapContractPos` 初始化和 `m_positions` 缓存的今仓/昨仓拆分正确
- 添加日志验证：初始化时打印原始 CTP 字段值，便于后续排查

## Capabilities

### New Capabilities
- `ctp-position-yesterday-fix`: 修复 CTP 昨仓量计算，使用 `Position - TodayPosition` 替代 `YdPosition`

### Modified Capabilities

## Impact

- `gateway/plugins/ctp/src/ctp_td_plugin.cpp`: `ConvertPosition()` 修改 yesterday_volume 计算
- `gateway/src/counter_bridge.cpp`: `g_mapContractPos` 初始化逻辑受益（无需修改，上游数据正确后自动修复）
- 影响所有依赖 `SetCombOffsetFlag()` 的平仓/平今/平昨判断
