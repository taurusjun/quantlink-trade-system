## 1. DailyInit 结构体和 Load/Save 重写

- [x] 1.1 重写 `daily_init.go`: 增加 StrategyID/OrigBaseName1/OrigBaseName2 字段，重命名为 LoadMatrix2/SaveMatrix2，实现 C++ header+data 格式
- [x] 1.2 重写 `daily_init_test.go`: 测试 C++ 格式的 round-trip、C++ 生成文件解析、strategyID 不匹配错误、文件不存在

## 2. 调用方更新

- [x] 2.1 更新 `pairwise_arb.go` handleSquareoffLocked: SaveDailyInit → SaveMatrix2，传入 StrategyID 和合约名
- [x] 2.2 更新 `pairwise_arb_test.go` TestPairwiseArb_HandleSquareoff: LoadDailyInit → LoadMatrix2，验证新字段
- [x] 2.3 更新 `main.go`: LoadDailyInit → LoadMatrix2，传入 strategyID

## 3. 数据文件和验证

- [x] 3.1 转换 `deploy_new/data/daily_init.92201` 为 C++ header+data 格式
- [x] 3.2 运行 go test 验证全部通过
- [x] 3.3 运行 go build 验证编译通过
