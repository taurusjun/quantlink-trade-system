## Why

当前 `data_new/` 目录将模拟盘和实盘的配置、运行时状态（`daily_init.*`）混在一起。即使 `data_new/` 按模式分离，`deploy_new/data/` 仍然共享同一份 `daily_init.*`。sim 运行产生的持仓状态会污染 live 的初始化数据，反之亦然。需要从源头（data_new）到运行时（deploy_new）彻底隔离两种模式的配置和数据。

## What Changes

- 重组 `data_new/` 目录为 `common/` + `sim/` + `live/` 三层结构（已完成）
- **BREAKING**: `build_deploy_new.sh` 新增 `--mode sim|live` 参数，合并逻辑改为先复制 common 再 overlay 模式目录（已完成）
- `deploy_new/data/` 按模式分目录存储运行时状态：`data/sim/daily_init.*` 和 `data/live/daily_init.*`
- `start_gateway.sh` 写 `.gateway_mode` 文件记录当前运行模式
- `start_strategy.sh` 读 `.gateway_mode`，传 `-dataDir` 参数给 trader
- Go trader `main.go` 新增 `-dataDir` flag，替代硬编码的 `./data`

## Capabilities

### New Capabilities
- `mode-aware-deploy`: 按 sim/live 模式分离配置和数据的部署能力，包括 data_new 三层结构、build_deploy_new.sh 模式感知合并、deploy_new 运行时数据按模式隔离

### Modified Capabilities
（无已有 spec 需要修改）

## Impact

- `data_new/` 目录结构重组（已完成）
- `scripts/build_deploy_new.sh` 合并逻辑（已完成 + deploy_new data 分目录）
- 生成的 `deploy_new/scripts/start_gateway.sh` 模板更新（写 .gateway_mode）
- 生成的 `deploy_new/scripts/start_strategy.sh` 模板更新（读 .gateway_mode，传 -dataDir）
- `tbsrc-golang/cmd/trader/main.go` 新增 `-dataDir` flag
