# Proposal: 重新部署时保留 deploy_new/data 文件夹

## 问题

`scripts/build_deploy_new.sh --clean` 执行 `rm -rf deploy_new/`，会删除 `deploy_new/data/` 下的运行时数据（如 `daily_init.92201`），导致策略的 avgPx、昨仓等状态丢失。

## 方案

修改 `--clean` 逻辑：在清理 deploy_new 时，保留 `data/` 子目录。只清理 bin/、scripts/、web/、lib/、config/、controls/、models/ 等编译产物和配置，不删除运行时数据。

## 影响范围

- `scripts/build_deploy_new.sh`：修改 `--clean` 清理逻辑
