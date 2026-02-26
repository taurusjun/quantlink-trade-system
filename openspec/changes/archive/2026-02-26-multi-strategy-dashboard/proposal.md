# Proposal: 多策略综合监控页面

## Why

当前 Java 交易系统只有单策略 dashboard，无法统一监控多个策略实例。原系统（amois.intranet 监控页面）是完整的综合监控管理中心，包含 9 大功能区域。新系统页面需要与原系统完全一致。

## What Changes

新增后端聚合服务 + 前端 overview 页面，完全对齐原系统 9 个区域布局。

### 9 大区域

1. **顶部控制栏** — stopAll、品种/策略/状态筛选、清除筛选
2. **策略列表表格**（主体）— Status/Alert/AT/Pro/ID/IP/ModelFile/StrategyType/Key/val/L1/L2/PNL/Time/Information/操作
3. **Account Table**（右上）— Pro/Ex/TotalAsset/AvailCash/Margin/Risk
4. **PyBot Tables**（右中）— state/ID/Model/Value/Log
5. **Spread Trades**（底左）— ModelFile/S/Qty/Spread/Time/Pro
6. **Orders**（底中左）— Symbol/S/Qty/Price/Modelfile/ID/pro + cxl All
7. **Position Table**（底中）— Symbol/Pos/CXLRio/Pro
8. **Fills**（底中右）— Time/Symbol/S/Price/Qty/ID/Pro
9. **Option Table**（底右）— product/GroupDel/Contract/Delta1/pos/SDelta

### 后端架构

新增 `OverviewServer`（独立端口 8080），聚合所有 trader 实例数据：
- 轮询各 trader REST API 采集快照
- 聚合账户资金、持仓、成交、挂单数据
- WebSocket 推送聚合后的完整数据给前端
- REST API 转发控制命令到对应 trader

## Capabilities

- **overview-page**: Overview 综合监控页面（9 区域 + 后端聚合）

## Impact

- **新增文件**: OverviewServer.java, OverviewSnapshot.java, overview.html
- **修改文件**: 启动脚本
- **端口**: 8080（overview）, 9201-9210（各 trader）
