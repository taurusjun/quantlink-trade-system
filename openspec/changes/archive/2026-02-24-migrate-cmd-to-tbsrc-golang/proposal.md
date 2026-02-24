## Why

`golang/` 目录即将弃用，`tbsrc-golang/` 是新的 Go 代码目录。`build_deploy_new.sh` 已改为从 `tbsrc-golang/` 编译，但 `webserver`、`backtest`、`backtest_optimize` 三个 cmd 还在 `golang/cmd/` 中，导致编译失败。需要将它们迁移过来。

## What Changes

- 将 `golang/cmd/webserver/` 复制到 `tbsrc-golang/cmd/webserver/`，调整 import path
- 将 `golang/cmd/backtest/` 复制到 `tbsrc-golang/cmd/backtest/`，调整 import path
- 将 `golang/cmd/backtest_optimize/` 复制到 `tbsrc-golang/cmd/backtest_optimize/`，调整 import path
- `backtest` 和 `backtest_optimize` 依赖 `pkg/backtest`，需同时迁移该 package
- 模块名从 `github.com/yourusername/quantlink-trade-system` 改为 `tbsrc-golang`
- `build_deploy_new.sh` 中的可选编译（backtest/backtest_optimize）无需改动（已指向 tbsrc-golang）

## Capabilities

### New Capabilities

（无新增能力，属于现有代码迁移）

### Modified Capabilities

（无 spec 级别的行为变更，仅代码位置和 import path 变更）

## Impact

- **代码**: `tbsrc-golang/cmd/` 新增 3 个 cmd，`tbsrc-golang/pkg/` 新增 backtest package
- **构建**: `build_deploy_new.sh` 将能完整编译所有 Go 组件
- **依赖**: import path 从 `github.com/yourusername/quantlink-trade-system/pkg/...` 改为 `tbsrc-golang/pkg/...`
- **测试**: 迁移后需验证 `go build` 和 `go test` 通过
