# Tasks: 修复 shutdown 保存 + origBaseName 写入

## Fix 1: Shutdown 无条件保存

- [x] 1.1 移除 main.go 中 SIGTERM handler 的 `IsActive()` 守卫，对齐 C++ 无条件 HandleSquareoff
- [x] 1.2 验证 trader 停止后 daily_init 文件已保存（avgPx 更新）

## Fix 2: OrigBaseName 字段

- [x] 2.1 在 `instrument.Instrument` 结构体中添加 `OrigBaseName` 字段
- [x] 2.2 在 main.go 中从 controlFile 配置赋值 `inst.OrigBaseName`
- [x] 2.3 修改 `pairwise_arb.go` SaveMatrix2 使用 `OrigBaseName` 替代 `Symbol`
- [x] 2.4 更新测试: `newTestInstrument` 设置 OrigBaseName，断言使用 OrigBaseName

## 附带清理

- [x] 3.1 注释掉 config_CHINA.92201.cfg 和 .92202.cfg 中未使用的 C++ 遗留字段

## 验证

- [x] 4.1 Go 编译通过
- [x] 4.2 单元测试通过 (`go test ./pkg/...`)
- [x] 4.3 实盘测试: trader 启动 → 停止 → daily_init 已保存且 origBaseName 正确
