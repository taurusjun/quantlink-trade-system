# Design: 重新部署时保留 deploy_new/data 文件夹

## 当前行为

`build_deploy_new.sh` 第 111-114 行：

```bash
if [ "$CLEAN_BUILD" = true ]; then
    log_info "清理 deploy_new 目录..."
    rm -rf "${DEPLOY_DIR}"
fi
```

`--clean` 会删除整个 `deploy_new/` 目录，包括 `data/live/daily_init.*` 等运行时数据。

## 修改方案

将 `rm -rf "${DEPLOY_DIR}"` 改为选择性清理：删除编译产物目录（bin、lib、scripts、web、config、controls、models、log），保留 `data/` 和 `ctp_flow/`。

```bash
if [ "$CLEAN_BUILD" = true ]; then
    log_info "清理 deploy_new 目录（保留 data/ 和 ctp_flow/）..."
    for clean_dir in bin lib scripts web config controls models log; do
        rm -rf "${DEPLOY_DIR}/${clean_dir}"
    done
fi
```

## 不修改的部分

- 数据复制逻辑（第 700-715 行）已正确处理：只复制不存在的文件（`if [ ! -f "$target" ]`），无需修改。
- 非 `--clean` 模式不受影响：不删除任何内容，只覆盖/新增。
