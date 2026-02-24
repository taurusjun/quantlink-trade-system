## Why

当前系统已完成从 NATS/gRPC 5 进程架构到 SysV MWMR SHM 直连架构的迁移，但缺少一份面向开发者的系统架构全景文档。现有文档分散在多个实施报告和技术规格中（`tbsrc-golang_v2_架构更新`、`hftbase_MWMR_Go复刻技术规格`、`DEPLOY_GUIDE`），新开发者需要阅读大量文档才能理解整体系统。需要一份统一的架构理解文档，帮助后续开发者快速建立全局认知。

## What Changes

- 新增 `docs/系统分析/系统架构全景_2026-02-24.md`，作为系统架构的入口级文档
- 覆盖：组件职责、SysV MWMR SHM 队列设计、消息结构体、数据流（行情/订单/回报/客户端注册）、Go 包职责、C++ 组件职责、启动流程、配置体系、关键设计决策
- 不修改任何代码或配置，纯文档新增

## Capabilities

### New Capabilities
- `architecture-overview`: 系统架构全景文档，覆盖组件、数据流、SHM 设计、消息格式、启动流程、配置体系、设计决策

### Modified Capabilities

## Impact

- `docs/系统分析/系统架构全景_2026-02-24.md` — 新增文档
- `docs/README.md` — 需要在系统分析章节添加新文档索引
