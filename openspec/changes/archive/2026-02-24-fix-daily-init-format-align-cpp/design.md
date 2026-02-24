## Context

C++ `SaveMatrix2` 输出格式（PairwiseArbStrategy.cpp:673-676）：

```
StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
92201 0 96.671581 ag2603 ag2605 83 -83
```

C++ `LoadMatrix2` 解析逻辑（PairwiseArbStrategy.cpp:112-144）：
1. 第 1 行 Tokenize 为 header 列名数组
2. 第 2+ 行 Tokenize，`Tokens[0]` 为 strategyID，后续字段按 header 索引存入 `map<string,string>`
3. 调用方用 `row["avgPx"]`、`row["ytd1"]` 等 key 取值

Go 当前实现完全自创了一个每行一个数值的格式，方法名也不同。

## Goals / Non-Goals

**Goals:**
- `LoadMatrix2` / `SaveMatrix2` 与 C++ 文件格式 100% 互操作
- 方法名与 C++ 一致
- DailyInit 结构体包含所有 C++ 字段

**Non-Goals:**
- 不实现 C++ 的 `flock` 文件锁（Go 单进程写入，不需要）
- 不实现多策略行支持（当前一个文件对应一个 strategyID，与 C++ 一致）
- 不改变 `DailyInitPath` 路径生成逻辑

## Decisions

### Decision 1: DailyInit 结构体增加字段

```go
type DailyInit struct {
    StrategyID    int32
    Netpos2day1   int32    // header: "2day"
    AvgSpreadOri  float64  // header: "avgPx"
    OrigBaseName1 string   // header: "m_origbaseName1"
    OrigBaseName2 string   // header: "m_origbaseName2"
    NetposYtd1    int32    // header: "ytd1"
    NetposAgg2    int32    // header: "ytd2"
}
```

字段顺序与 C++ header 列顺序一致：`StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2`

### Decision 2: LoadMatrix2 按 header 列名索引

与 C++ 一致，用 header 列名做 key 取值，而不是按固定位置索引。这样即使列顺序变化也能正确解析。

### Decision 3: SaveMatrix2 使用 C++ 完全一致的 header 字符串

硬编码 header 为 `"StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 "`（注意末尾空格，与 C++ line 673 一致）。

avgPx 使用 `%f` 格式（Go 默认 6 位小数，与 C++ `ios::fixed` 默认精度一致）。

## Risks / Trade-offs

- [风险] 现有 deploy_new/data/daily_init.92201 是旧格式 → 需同步更新为 C++ 格式
- [风险] pairwise_arb.go 中 SaveMatrix2 需要传入合约名 → 已有 `pas.Leg1Symbol` / `pas.Leg2Symbol` 可用
