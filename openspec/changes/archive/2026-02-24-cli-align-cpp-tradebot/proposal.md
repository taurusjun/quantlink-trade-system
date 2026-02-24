## Why

Go trader 当前使用自创的 `-config trader.92201.yaml` YAML 配置方式，与 C++ TradeBot 的 `--controlFile` + `--configFile` + `--strategyID` 方式完全不同。需要对齐为 C++ 原方式以保持运维兼容性，使用同一套 control/model/config 文件。

## What Changes

- **BREAKING**: `main.go` CLI 参数从 `-config`/`-data` 替换为 `--controlFile`/`--configFile`/`--strategyID` 等 C++ 风格参数
- 新增 `pkg/config/control_file.go`: 解析 C++ controlFile 格式（单行空格分隔）
- 新增 `pkg/config/cfg_file.go`: 解析 C++ .cfg INI 格式（SHM keys、PRODUCT 等）
- 新增 `pkg/config/model_file.go`: 解析 C++ model file（阈值参数 BEGIN_PLACE/ALPHA 等）
- 修改 `config.go`: 从 C++ 三文件组合构建 Config 结构体（保持内部 Config 结构不变）
- daily_init 路径硬编码为 `../data/daily_init.<strategyID>`（与 C++ 完全一致）
- 创建 `data_new/config/config_CHINA.92201.cfg` 格式配置文件
- 创建 `data_new/data/daily_init.92201` 文件
- 更新 `build_deploy_new.sh` 和启动脚本适配新 CLI

## Capabilities

### New Capabilities
- `cpp-config-parser`: 解析 C++ TradeBot 原格式配置文件（controlFile、configFile、modelFile）

### Modified Capabilities
（无已有 spec 需要修改）

## Impact

- `tbsrc-golang/cmd/trader/main.go` — CLI 入口重写
- `tbsrc-golang/pkg/config/` — 新增 3 个解析器，修改 Load 入口
- `data_new/config/` — 新增 .cfg 文件、保留 YAML（向后兼容）
- `data_new/data/` — 新增 daily_init 文件
- `scripts/build_deploy_new.sh` — 更新启动脚本生成
- 所有现有测试需通过
