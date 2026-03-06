## Overview

创建 `MultiClientStoreShmReader.java`，1:1 对齐 C++ `hftbase/Ipc/include/multiclientstoreshmreader.h`。

## C++ 原代码分析

**文件**: `hftbase/Ipc/include/multiclientstoreshmreader.h`
**类**: `MultiClientStoreShmReader<MD, REQ, RESP, MAXSIZE>`
**实例化**: `Connector` 中 `typedef MultiClientStoreShmReader<MarketUpdateNew, RequestMsg, ResponseMsg, MAX_ORS_CLIENTS> ShmMgr;`

### C++ 字段

| C++ 字段 | 类型 | Java 映射 |
|----------|------|-----------|
| `m_mdClients[MAXSIZE]` | `MDClient<MdShmQ, MD>*[]` | `MWMRQueue[] mdQueues` |
| `m_mdWithEndPacketClients[MAXSIZE]` | `MDWithEndPacketClient*[]` | `MWMRQueue[] mdEndPktQueues` |
| `m_reqClients[MAXSIZE]` | `ReqShmQ*[]` | `MWMRQueue[] reqQueues` |
| `m_respClients[MAXSIZE]` | `RespShmQ*[]` | `MWMRQueue[] respQueues` |
| `clientStores` | `map<size_t, LocklessShmClientStore*>` | `Map<Integer, ClientStore> clientStores` |
| `m_mdClientCount` | `uint32_t` | `int mdClientCount` |
| `m_mdWithEndPacketClientCount` | `uint32_t` | `int mdEndPktClientCount` |
| `m_reqClientCount` | `uint32_t` | `int reqClientCount` |
| `m_respClientCount` | `uint32_t` | `int respClientCount` |
| `m_defaultClientStoreKey` | `size_t` | `int defaultClientStoreKey` |
| `m_active/m_mdActive/m_orsRequestActive/m_orsResponseActive` | `bool` | `volatile boolean` |
| `m_threadHandler/m_mdThread/m_orsRequestThread/m_orsResponseThread` | `std::thread` | `Thread` |
| `m_response_queue_to_exchange_map` | (在 Connector 中) | 不在此类 |

### C++ 方法 → Java 方法

| C++ 方法 | Java 方法 | 说明 |
|----------|-----------|------|
| `initClientStore(key)` | `initClientStore(int key)` | 创建/连接 ClientStore |
| `registerMDClient(key, size)` | `registerMDClient(int key, int size)` | 注册行情队列 |
| `registerMDWithEndPacketClient(key, size)` | `registerMDWithEndPacketClient(int key, int size)` | 注册带 endPkt 行情队列 |
| `registerRequestClient(key, size, &clientId, csKey)` | `registerRequestClient(int key, int size, int csKey)` → 返回 clientId | 注册请求队列 + 分配 clientId |
| `registerResponseClient(key, size)` | `registerResponseClient(int key, int size)` | 注册回报队列 |
| `getRequestClient(key, size)` | `getRequestClient(int key, int size)` | 获取请求队列（不分配 clientId） |
| `loopMD()` | `loopMD()` | 轮询行情（ROUND_ROBIN 模式） |
| `loopMD_until_endpacket()` | `loopMDUntilEndpacket()` | 轮询行情（UNTIL_ENDPACKET 模式） |
| `loopRequest()` | `loopRequest()` | 轮询请求 |
| `loopResponse()` | `loopResponse()` | 轮询回报 |
| `startMonitorAll()` | `startMonitorAll()` | 组合轮询（阻塞） |
| `startMonitorAsyncAll()` | `startMonitorAsyncAll()` | 组合轮询（异步线程） |
| `startMonitorMarketData()` | `startMonitorMarketData()` | 仅行情轮询 |
| `startMonitorAsyncMarketData()` | `startMonitorAsyncMarketData()` | 行情异步线程 |
| `startMonitorMarketDataAndResponse()` | `startMonitorMarketDataAndResponse()` | 行情+回报轮询 |
| `startMonitorAsyncMarketDataAndResponse()` | `startMonitorAsyncMarketDataAndResponse()` | 行情+回报异步 |
| `startMonitorMarketDataAndResponseAndRequest()` | `startMonitorMarketDataAndResponseAndRequest()` | 全部轮询 |
| `startMonitorORSRequest()` | `startMonitorORSRequest()` | 请求轮询 |
| `startMonitorAsyncORSRequest()` | `startMonitorAsyncORSRequest()` | 请求异步 |
| `startMonitorORSResponse()` | `startMonitorORSResponse()` | 回报轮询 |
| `startMonitorAsyncORSResponse()` | `startMonitorAsyncORSResponse()` | 回报异步 |
| `startMonitorORSRequestHighPerf()` | `startMonitorORSRequestHighPerf()` | 高性能请求轮询 |
| `totalSHMRequestQueues()` | `totalSHMRequestQueues()` | 请求队列总数 |
| `getMaxClientId(csKey)` | `getMaxClientId(int csKey)` | 获取最大 clientId |
| `getReqMsgQueueForClient(key, size, clientId)` | `getReqMsgQueueForClient(int key, int size, int clientId)` | 按 clientId 获取请求队列 |
| `shutdown()` | `shutdown()` | 停止线程+清理 |
| `stopMonitor()` | `stopMonitor()` | 停止轮询 |
| `waitForCompletion*()` | `waitForCompletion*()` | 等待线程结束 |

### 回调机制

C++ 使用 `DEFINE_SIGNAL` / `EMIT_PARAM` 宏实现信号-槽模式：
- `MarketUpdateAvailable` → Java: `MDCallback.onMarketUpdate(MemorySegment)`
- `ORSRequestAvailable` → Java: `ORSRequestCallback.onRequest(MemorySegment)`
- `ORSResponseAvailable` → Java: `ORSResponseCallback.onResponse(MemorySegment, int queueIndex)`
- `MDNoUpdateAvailable` → Java: 不实现（`_SIGNAL_ON_MD_EMPTYQ` 条件编译，默认关闭）
- `ORSNoRequestAvailable` → Java: 不实现（`_SIGNAL_ON_EMPTYQ` 条件编译，默认关闭）

### MDWithEndPacketClient 对齐

C++ `MDWithEndPacketClient` 内部状态：
- `contains_new_data`: 是否已预取数据
- `last_packet_was_endpacket`: 上一个包是否 endPkt
- `data`: 预取的数据缓存
- `fetch_data_if_possible_from_queue()`: 尝试从队列取数据

Java 需要内部类 `MDEndPktClient` 封装相同状态。

### getBestEndPacketClientAsPerTimestampSequencing

C++ 在 endPkt 模式下选择 timestamp 最小的队列优先处理，Java 需要 1:1 对齐。

## 设计决策

### 泛型 vs 固定类型

C++ 使用模板 `<MD, REQ, RESP, MAXSIZE>`。Java 中 MD/REQ/RESP 都是 `MemorySegment`（Panama FFI），无需泛型，队列大小由 `MWMRQueue` 内部管理。`MAXSIZE` 作为构造参数。

### 文件位置

`tbsrc-java/src/main/java/com/quantlink/trader/shm/MultiClientStoreShmReader.java`

与 `MWMRQueue.java`、`ClientStore.java`、`SysVShm.java` 同包（对齐 C++ `hftbase/Ipc/include/`）。

### 阶段 1 范围

- 创建完整的 `MultiClientStoreShmReader.java`
- 编译通过
- 不修改 `Connector.java`（阶段 2）
