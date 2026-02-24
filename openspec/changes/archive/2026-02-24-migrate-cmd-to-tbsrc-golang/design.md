## Context

`tbsrc-golang/` 使用模块名 `tbsrc-golang`，`golang/` 使用模块名 `github.com/yourusername/quantlink-trade-system`。三个 cmd 需要迁移：

| cmd | 内部依赖 | 迁移难度 |
|-----|---------|---------|
| `webserver` | 无（纯 stdlib） | 低 — 直接复制 |
| `backtest` | `pkg/backtest` | 中 — 需同时迁移 pkg |
| `backtest_optimize` | `pkg/backtest` | 中 — 同上 |

`pkg/backtest` 是 backtest/backtest_optimize 的共同依赖，需要一起迁移。

## Goals / Non-Goals

**Goals:**
- 将 webserver、backtest、backtest_optimize 迁移到 `tbsrc-golang/cmd/`
- 将 `pkg/backtest` 迁移到 `tbsrc-golang/pkg/backtest/`
- 所有 import path 改为 `tbsrc-golang/pkg/...`
- `build_deploy_new.sh` 完整编译通过
- `go build` 和 `go test` 通过

**Non-Goals:**
- 不重构 backtest 代码逻辑
- 不迁移 golang/cmd/ 下其他 demo/test cmd（如 strategy_demo、indicator_demo 等）
- 不删除 golang/ 目录（保持向后兼容）

## Decisions

**决策 1: 复制而非移动**
- 将文件从 `golang/` 复制到 `tbsrc-golang/`，不删除原文件
- 理由：`golang/` 仍保留作为参考，避免破坏可能的旧引用

**决策 2: import path 批量替换**
- `github.com/yourusername/quantlink-trade-system/pkg/` → `tbsrc-golang/pkg/`
- 只替换迁移文件中的 import，不修改 golang/ 原文件

**决策 3: pkg/backtest 依赖链检查**
- 需检查 `pkg/backtest` 自身的 import，看是否依赖其他未迁移的 pkg
- 如有额外依赖，需一并迁移

## Risks / Trade-offs

- [Risk] `pkg/backtest` 可能依赖 tbsrc-golang 中尚不存在的 pkg → 编译前先检查依赖链，必要时迁移更多 pkg
- [Risk] go.mod 依赖不一致 → 迁移后运行 `go mod tidy` 确保依赖一致
