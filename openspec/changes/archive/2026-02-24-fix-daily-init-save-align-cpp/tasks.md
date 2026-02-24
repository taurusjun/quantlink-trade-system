## 1. 修正 HandleSquareoff 保存逻辑

- [x] 1.1 修改 `pairwise_arb.go` handleSquareoffLocked() 中 daily_init 保存: NetposYtd1 = NetposPass (total), Netpos2day1 = 0
- [x] 1.2 添加 C++ SaveMatrix2 对照注释

## 2. 移除 main.go 重复保存

- [x] 2.1 删除 main.go shutdown 中的 daily_init 保存块（第 268-279 行）
- [x] 2.2 更新 shutdown 步骤编号注释

## 3. 测试

- [x] 3.1 新增 TestPairwiseArb_HandleSquareoff_DailyInit 测试用例，验证 NetposYtd1=total, Netpos2day1=0
- [x] 3.2 运行 `go test -race ./pkg/strategy/... ./pkg/config/...` 确认通过
