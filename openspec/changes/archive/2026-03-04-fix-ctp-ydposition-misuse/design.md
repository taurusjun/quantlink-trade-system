## Context

CTP API 对 SHFE 合约的持仓查询（`ReqQryInvestorPosition`）返回**多条记录**：今仓一条、昨仓一条。每条记录的字段含义：

| 字段 | 今仓记录 | 昨仓记录 |
|------|---------|---------|
| `Position` | 当前今仓量 | 当前剩余昨仓量（已扣减今日平昨） |
| `TodayPosition` | 今仓量（= Position） | 0 |
| `YdPosition` | 0 | **昨日结算时持仓量（不变，不扣减今日平昨）** |

当前 `ConvertPosition()` 使用 `YdPosition` 作为 `yesterday_volume`，导致昨仓被高估。

**实盘证据** (2026-03-04)：
- ag2606 实际持仓 Short=6（今5+昨1），CTP 返回 YdPosition=8
- g_mapContractPos 累加后 Short=13（T:5+Y:8），SetCombOffsetFlag 认为有 8 手昨仓可平
- 实际昨仓只有 1 手 → CTP 返回 ErrorID 51

## Goals / Non-Goals

**Goals:**
- 修复 `ConvertPosition()` 的 `yesterday_volume` 计算
- 确保 `g_mapContractPos` 和 `m_positions` 的今仓/昨仓拆分正确
- 消除 ErrorID 51（平仓量超过持仓量）

**Non-Goals:**
- 不修改 `SetCombOffsetFlag()` 逻辑（上游数据正确后无需修改）
- 不修改 `UpdatePositionFromCTP()` 的累加逻辑（CTP 多条记录累加是正确的）

## Decisions

**Decision 1: 使用 `Position - TodayPosition` 替代 `YdPosition`**

`ConvertPosition()` L1186:
```cpp
// 修复前
pos_info.yesterday_volume = ctp_pos->YdPosition;

// 修复后
pos_info.yesterday_volume = ctp_pos->Position - ctp_pos->TodayPosition;
```

理由：`Position` 是 CTP 返回的当前实际持仓量（已扣减今日平仓），`TodayPosition` 是今仓量，差值即为实际剩余昨仓。这对今仓记录和昨仓记录都正确：
- 今仓记录: `Position=5, TodayPosition=5` → `yesterday=0` ✓
- 昨仓记录: `Position=1, TodayPosition=0` → `yesterday=1` ✓

备选方案：仅在 `g_mapContractPos` 初始化时修复。不采用，因为 `m_positions` 缓存也有同样问题，修复 `ConvertPosition` 是根治方案。

## Risks / Trade-offs

- **[风险] 非 SHFE 交易所行为差异** → CTP 对非 SHFE 交易所不区分今昨仓，`Position - TodayPosition` 公式仍然正确（TodayPosition=0 时 yesterday=Position）
- **[风险] CTP Position 字段为负** → CTP 不会返回负值 Position，但增加防护 `max(0, Position - TodayPosition)`
