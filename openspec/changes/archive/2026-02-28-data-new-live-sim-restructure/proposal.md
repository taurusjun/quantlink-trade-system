## Why

data_new 目录中 controls 放在 common/ 下，不区分 live/sim 环境，导致 control 文件中的 model 路径需要运行时覆盖才能指向正确的环境专属模型。deploy_java 也使用扁平的 controls/models/data 目录，无法直观区分环境。同时 CTP 实盘风控参数（UPNL_LOSS=3000, MAX_SIZE=2）与实际 70~80 手持仓严重不匹配，频繁触发止损。

## What Changes

- 将 `data_new/common/controls/` 删除，controls 分散到 `data_new/live/controls/` 和 `data_new/sim/controls/`
- live control 文件内 model 路径指向 `./live/models/model.*`，sim 指向 `./sim/models/model.*`
- deploy_java 目录结构从 `controls/` + `models/` + `data/` 改为 `live/{controls,models,data}` + `sim/{controls,models,data}`
- 更新 `build_deploy_java.sh` 合并逻辑：按 live/sim 分别部署
- 更新内嵌 `start_strategy.sh`：按 gateway_mode 选择 `live/` 或 `sim/` 目录，移除运行时 model 覆盖
- 更新内嵌 `start_gateway.sh`：从 `sim/controls/` 读取合约列表
- 更新内嵌 `start_all.sh`：按环境目录扫描 controls
- 调大 CTP 实盘风控参数匹配 70~80 手持仓

## Capabilities

### New Capabilities
- `live-sim-directory-structure`: data_new 和 deploy_java 的 live/sim 环境隔离目录结构

### Modified Capabilities

## Impact

- `data_new/common/controls/` — 删除
- `data_new/live/controls/{day,night}/` — 新建
- `data_new/sim/controls/{day,night}/` — 新建
- `data_new/live/models/model.ag2603.ag2605.par.txt.92201` — 更新风控参数
- `scripts/build_deploy_java.sh` — 合并逻辑 + 内嵌脚本更新
- `deploy_java/` — 运行时结构从扁平改为 live/sim 分离
