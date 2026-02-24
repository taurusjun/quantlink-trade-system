# Proposal: 更新 golang/ 路径为 tbsrc-golang/

## 背景

`golang/` 目录即将弃用，新的 Go 代码在 `tbsrc-golang/`。需要更新所有构建脚本和文档中的路径引用。

## 变更范围

1. `scripts/build_deploy_new.sh` — golang/ 路径改为 tbsrc-golang/
2. `/Users/user/PWorks/RD/CLAUDE.md` — 迁移表、构建命令路径更新
3. `.claude/CLAUDE.md` — 迁移表、架构描述、构建命令、文件结构更新
4. `.gitignore` — 添加 tbsrc-golang/ 对应条目

## 不改的文件

- `scripts/build_deploy.sh`（保持原样）
- `scripts/archive/` 下的归档脚本
- config YAML 文件
