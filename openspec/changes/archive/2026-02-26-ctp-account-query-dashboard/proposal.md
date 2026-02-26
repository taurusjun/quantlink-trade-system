## Why

Overview 页面 Account Table 显示 "TODO: CTP 资金查询" 占位文字，无法展示真实资金信息。需要实现从 counter_bridge 到前端的完整资金查询链路，使运维人员能实时监控账户资金、保证金和风险度。

## What Changes

- counter_bridge (C++) HTTP 端口从 8080 改为 **8082**（避免与 OverviewServer 8080 冲突）
- counter_bridge 新增通用 `/account` 端点，自动查询第一个可用 broker plugin（CTP 或 Simulator）
- OverviewServer (Java) 新增定时任务，每 10 秒 HTTP 查询 `localhost:8082/account`
- OverviewSnapshot 新增 `AccountRow` 数据模型，聚合到 overview 推送
- 前端 Account Table 从占位文字改为 Vue 数据绑定，展示资金详情

## Capabilities

### New Capabilities

- `account-query`: counter_bridge 通用资金查询 HTTP 端点，支持 CTP 和 Simulator 模式

### Modified Capabilities

- `overview-page`: Account Table 从占位改为实时数据绑定，新增 OverviewServer → counter_bridge 资金查询链路

## Impact

- **C++ 网关**: `gateway/src/counter_bridge.cpp` — 端口变更 + 新端点
- **Java 后端**: `OverviewServer.java`, `OverviewSnapshot.java` — 定时查询 + 数据模型
- **前端**: `overview.html` — Account Table 数据绑定
- **部署脚本**: `scripts/build_deploy_java.sh` — 端口注释更新
- **端口变更**: counter_bridge HTTP 8080 → 8082（**BREAKING** 如有外部依赖旧端口）
