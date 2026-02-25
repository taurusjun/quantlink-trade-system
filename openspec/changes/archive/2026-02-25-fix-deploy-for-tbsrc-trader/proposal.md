## Why

`build_deploy_new.sh` 生成的 `start_strategy.sh` 使用旧版 golang trader 的 CLI 接口（`--Live`, `--controlFile`, `--strategyID`），但 tbsrc-golang trader 只接受 `-config` 和 `-data`。deploy_new 中也缺少 tbsrc-golang 格式的配置文件。

## What Changes

- `build_deploy_new.sh` 中 `start_strategy.sh` 模板：改用 tbsrc-golang trader 的 `-config` 和 `-data` 参数
- `data_new/config/` 中添加 `trader.tbsrc.yaml`（每个策略一个配置文件，如 `trader.92201.yaml`）
- 更新合约从 ag2506/ag2512 到 ag2603/ag2605

## Capabilities

### New Capabilities
（无）

### Modified Capabilities
（无 spec 变更，仅部署配置适配）

## Impact

- `deploy_new/scripts/start_strategy.sh` — CLI 参数变更
- `data_new/config/` — 新增 tbsrc-golang 格式的策略配置文件
