## Why

Go 的 `LoadDailyInit` / `SaveDailyInit` 使用自创的每行一个数值格式，与 C++ `LoadMatrix2` / `SaveMatrix2` (PairwiseArbStrategy.cpp:112-144, 653-686) 的 header+data 空格分隔格式完全不一致。方法名也与 C++ 不一致。导致 C++ 生成的 daily_init 文件 Go 无法读取，Go 生成的文件 C++ 也无法读取。

## What Changes

- 方法名对齐 C++：`LoadDailyInit` → `LoadMatrix2`，`SaveDailyInit` → `SaveMatrix2`
- `LoadMatrix2` 解析 C++ 格式：第 1 行 header（列名），第 2+ 行按 strategyID 索引的空格分隔数据
- `SaveMatrix2` 输出 C++ 格式：header 行 `"StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2"` + data 行
- `DailyInit` 结构体增加 `StrategyID`、`OrigBaseName1`、`OrigBaseName2` 字段
- 更新所有调用方（main.go、pairwise_arb.go）
- 更新现有的 `deploy_new/data/daily_init.92201` 为 C++ 格式
- 更新测试

## Capabilities

### New Capabilities

### Modified Capabilities
- `daily-init-save`: LoadDailyInit/SaveDailyInit 重命名为 LoadMatrix2/SaveMatrix2，格式对齐 C++

## Impact

- `tbsrc-golang/pkg/config/daily_init.go`: 重写 Load/Save，重命名方法
- `tbsrc-golang/pkg/config/daily_init_test.go`: 测试更新为 C++ 格式
- `tbsrc-golang/pkg/strategy/pairwise_arb.go`: 调用方更新
- `tbsrc-golang/cmd/trader/main.go`: 调用方更新
- `deploy_new/data/daily_init.92201`: 转换为 C++ 格式
- `data_new/data/daily_init.92201`: 同步更新（如存在）
