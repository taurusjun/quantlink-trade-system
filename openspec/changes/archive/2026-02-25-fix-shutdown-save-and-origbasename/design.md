# Design: 修复 shutdown 保存 + origBaseName 写入

## Fix 1: Shutdown 无条件保存

### 变更点

**文件**: `tbsrc-golang/cmd/trader/main.go`

移除 SIGTERM handler 中的 `if pas.IsActive()` 守卫，直接调用 `pas.HandleSquareoff()`。

**C++ 对照**: `tbsrc/main/main.cpp` `Squareoff()` 信号处理函数无条件调用 `Strategy->HandleSquareoff()`。

### 行为变化

- Before: 策略未激活时 SIGTERM 不保存 daily_init
- After: SIGTERM 始终保存 daily_init（与 C++ 一致）

## Fix 2: OrigBaseName 字段

### 数据流

```
controlFile (BaseName/SecondName)
    → main.go: inst.OrigBaseName = controlCfg.BaseName
    → pairwise_arb.go: SaveMatrix2 使用 OrigBaseName
    → daily_init 文件: ag_F_3_SFE（正确）
```

### 变更点

1. **instrument.go**: 添加 `OrigBaseName string` 字段
2. **main.go**: 从 controlFile 配置赋值 `inst1.OrigBaseName = controlCfg.BaseName`
3. **pairwise_arb.go**: `OrigBaseName1: pas.Inst1.OrigBaseName`（替代 `Symbol`）
4. **pairwise_arb_test.go**: 测试 helper 设置 OrigBaseName，断言使用 OrigBaseName

## 附带清理: config_CHINA 未使用字段

注释掉 `config_CHINA.92201.cfg` 和 `.92202.cfg` 中 10 个 C++ hftbase 遗留字段（Go 新系统未使用）。
