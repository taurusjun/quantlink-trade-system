# Tasks

- [x] 修改 `scripts/build_deploy_new.sh` 的 `--clean` 逻辑：将 `rm -rf "${DEPLOY_DIR}"` 改为选择性清理编译产物目录，保留 `data/` 和 `ctp_flow/`
