# Spec: symbolID 路由对齐 C++

## 概述

将 Java trader 的行情分发从 symbol 字符串路由改为 symbolID 数组索引，完全对齐 C++ 原代码。

### Requirement: md_shm_feeder symbolID 分配

md_shm_feeder 按字母排序分配 symbolID，与 C++ Connector 的 std::set 排序一致。

### Requirement: Java symbolID 数组索引

CommonClient.sendINDUpdate 使用 symbolID 从数组直接索引 SimConfig 和 Instrument，O(1) 复杂度。

### Requirement: CTP endPkt 修正

CTP OnRtnDepthMarketData 中 m_endPkt 必须设为 0（中国期货每个 snapshot 是完整包）。

### Requirement: 构建脚本分离

build_deploy_new.sh 仅包含 Go + C++，Java 使用 build_deploy_java.sh。
