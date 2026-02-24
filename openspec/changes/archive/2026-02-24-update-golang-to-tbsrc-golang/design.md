# Design: 更新 golang/ 路径为 tbsrc-golang/

## 方案

纯文本替换，无逻辑变更。

### build_deploy_new.sh

| 行 | 旧值 | 新值 |
|----|------|------|
| L102 | `mkdir -p "${DEPLOY_DIR}/golang/web"` | `mkdir -p "${DEPLOY_DIR}/web"` |
| L109-111 | `golang/web` 复制逻辑 | `tbsrc-golang/web` → `web/` |
| L169 | `cd "${PROJECT_ROOT}/golang"` | `cd "${PROJECT_ROOT}/tbsrc-golang"` |
| L647-648 | `golang/web/` 显示 | `web/` 显示 |

### CLAUDE.md (root)

- 迁移表: `golang/pkg/strategy/` → `tbsrc-golang/pkg/strategy/`
- 构建命令: `build_deploy.sh` → `build_deploy_new.sh`
- Go build/test 路径: `golang` → `tbsrc-golang`

### .claude/CLAUDE.md (project)

- 迁移对照表路径
- 架构组件 `golang/` → `tbsrc-golang/`
- 代码风格节标题
- 数据流名称 `golang_trader` → `trader`
- 构建命令路径
- 文件结构图: 新增 `tbsrc-golang/`，标注 `golang/` 已弃用

### .gitignore

- 添加 `tbsrc-golang/bin/`, `tbsrc-golang/pkg/proto/`, `tbsrc-golang/vendor/`, `tbsrc-golang/data/`, `tbsrc-golang/pkg/strategy/data/`
- 保留现有 `golang/` 条目（向后兼容）
