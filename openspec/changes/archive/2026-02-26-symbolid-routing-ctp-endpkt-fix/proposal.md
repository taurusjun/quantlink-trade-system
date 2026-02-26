# Proposal: symbolID 路由对齐 C++ + CTP endPkt 修复

## Why

Java 版 trader 的行情分发（CommonClient.sendINDUpdate）使用 symbol 字符串路由，
与 C++ 原代码的 symbolID 数组索引方式不一致。同时 md_shm_feeder 的 CTP 模式下
m_endPkt 字段错误设置为 1，导致 Java trader 丢弃所有 CTP 行情。

## What Changes

1. **md_shm_feeder symbolID 映射**: 新增 BuildSymbolIDMap，按字母排序分配 symbolID（与 C++ Connector 一致）
2. **md_shm_feeder CTP endPkt 修复**: CTP OnRtnDepthMarketData 回调中 m_endPkt=0（中国期货每个 snapshot 是完整包）
3. **Java symbolID 数组路由**: CommonClient.sendINDUpdate 从 symbol 字符串路由改为 symbolID 数组索引 O(1)
4. **ConfigParams/SimConfig 数组字段**: 新增 simConfigList[] / instruList[] 按 symbolID 索引
5. **TraderMain symbolID 构建**: 构建排序映射数组
6. **build_deploy_new.sh 清理**: 移除 Java 相关代码，Go 和 Java 使用独立脚本

## Capabilities

- symbolid-routing: symbolID 数组索引路由
- ctp-endpkt: CTP 行情 endPkt 修复
- build-script-separation: Go/Java 构建脚本分离

## Impact

- `gateway/src/md_shm_feeder.cpp` — symbolID 映射 + CTP endPkt 修复
- `tbsrc-java/.../CommonClient.java` — symbolID 路由
- `tbsrc-java/.../ConfigParams.java` — simConfigList 数组
- `tbsrc-java/.../SimConfig.java` — instruList 数组
- `tbsrc-java/.../TraderMain.java` — symbolID 构建
- `scripts/build_deploy_new.sh` — 移除 Java 部分
