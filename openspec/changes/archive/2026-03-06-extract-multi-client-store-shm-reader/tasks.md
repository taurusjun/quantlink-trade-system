# Tasks

## 1. 创建 MultiClientStoreShmReader.java

- [x] 创建文件 `tbsrc-java/src/main/java/com/quantlink/trader/shm/MultiClientStoreShmReader.java`
- [x] 1:1 对齐 C++ `hftbase/Ipc/include/multiclientstoreshmreader.h` 所有字段和方法
- [x] 包含回调接口定义（MDCallback / ORSRequestCallback / ORSResponseCallback）
- [x] 包含 MDEndPktClient 内部类
- [x] 编译通过

## 2. 翻译检查

- [x] 对比 C++ 原代码逐方法验证 — 27 个方法全部通过
- [x] 修复热循环 buf 分配问题 → 预分配为实例字段
- [x] 移除 C++ 中不存在的 isRequestQInitialized 赋值

## 3. 模拟测试（阶段 1）

- [x] 编译通过
- [x] 模拟测试：策略 92201 正常启动、无错误、无警告

## 4. 重构 Connector 使用 MultiClientStoreShmReader（阶段 2）

- [x] Connector 新增 `MultiClientStoreShmReader shmMgr` 字段，对齐 C++ `ShmMgr m_shmMgr` (connector.h:401)
- [x] `open()` 使用 shmMgr.initClientStore/registerMDClient/registerRequestClient/registerResponseClient
- [x] HandleUpdates/HandleOrderResponse 通过 shmMgr.setMDCallback/setORSResponseCallback 注册回调
- [x] `startAsync()` 委托 shmMgr.startMonitorAsyncMarketDataAndResponse()（对齐 connector.cpp:604）
- [x] `stop()` 委托 shmMgr.shutdown()（对齐 connector.cpp:680）
- [x] 移除 Connector 自身的 mdQueue/reqQueue/respQueue/clientStore 字段和轮询线程
- [x] requestQueue 从 shmMgr.getReqQueue(0) 获取（对齐 C++ m_requestQueue[exchId]）
- [x] createForTest 使用 ConnectorForTest 子类保留独立 SHM 管理能力

## 5. 翻译检查（阶段 2）

- [x] 对比 C++ connector.h/cpp 逐字段验证 — 13 处一致
- [x] 修复 handleUpdates 缺少 `m_endPkt == 1` 检查（对齐 connector.cpp:768）
- [x] 2 处轻微不一致已记录（exchType 硬编码 0, 缺少 m_all_clientIds 历史）
- [x] 6 处缺失均为非核心 LIVE 路径或性能优化接口（已标注）

## 6. 模拟测试（阶段 2）

- [x] 编译通过
- [x] 模拟测试：策略 92201 正常启动、无错误、无警告

## 7. 补齐 m_all_clientIds + GetOrderNumberWithNewClientId（阶段 3）

- [x] 新增 `Set<Integer> allClientIds` 字段，对齐 C++ `m_all_clientIds[MAX_EXCHANGE_COUNT][MAX_ORS_CLIENTS]` (connector.h:381)
- [x] 新增 `Config config` 字段，对齐 C++ `ConnectorConfig *m_cfg` (connector.h:398)
- [x] 构造函数初始化 `allClientIds.add(clientId)`，对齐 C++ connector.cpp:99-106
- [x] 实现 `getOrderNumberWithNewClientId(exchCode)`，对齐 C++ connector.cpp:1152-1182
- [x] `getUniqueOrderNumber` 溢出分支调用 `getOrderNumberWithNewClientId` 而非返回 -1
- [x] `handleOrderResponseFromShmMgr` 使用 `allClientIds.contains()` 替代 `clientIdMap.containsValue()`
- [x] 编译通过
- [x] 模拟测试：策略 92201 正常启动、无错误、无警告
