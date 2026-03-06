## ADDED Requirements

### Requirement: MultiClientStoreShmReader 类结构对齐

`MultiClientStoreShmReader` 必须 1:1 对齐 C++ `hftbase/Ipc/include/multiclientstoreshmreader.h`，包含：
- `MWMRQueue[] mdQueues` / `MDEndPktClient[] mdEndPktClients` / `MWMRQueue[] reqQueues` / `MWMRQueue[] respQueues`
- `Map<Integer, ClientStore> clientStores`
- 计数器: `mdClientCount` / `mdEndPktClientCount` / `reqClientCount` / `respClientCount`
- 控制标志: `active` / `mdActive` / `orsRequestActive` / `orsResponseActive`（volatile boolean）
- 线程: `threadHandler` / `mdThread` / `orsRequestThread` / `orsResponseThread`

### Requirement: initClientStore

`initClientStore(int shmKey)` 必须对齐 C++ `initClientStore(size_t clientStoreKey)`：
- 若 key 不在 `clientStores` 中，创建新 `ClientStore.open(shmKey)` 并加入映射
- 若 key 已存在，重新初始化（close + open）
- 更新 `defaultClientStoreKey`

### Requirement: registerMDClient

`registerMDClient(int shmKey, int queueSize)` 返回注册的 `MWMRQueue`，对齐 C++ `registerMDClient()`。
- 检查 `mdClientCount < maxSize`，超限抛异常
- 创建 `MWMRQueue.open()` 并存入 `mdQueues[mdClientCount++]`

### Requirement: registerMDWithEndPacketClient

`registerMDWithEndPacketClient(int shmKey, int queueSize)` 返回注册的 `MDEndPktClient`，对齐 C++ `registerMDWithEndPacketClient()`。
- 内部类 `MDEndPktClient` 封装 `MWMRQueue` + `containsNewData` / `lastPacketWasEndpacket` / `data`(MemorySegment 缓存)
- `fetchDataIfPossibleFromQueue()`: 尝试 dequeue，成功则设 `containsNewData=true`

### Requirement: registerRequestClient

`registerRequestClient(int shmKey, int queueSize, int clientStoreKey)` 返回分配的 clientId，对齐 C++ `registerRequestClient()`。
- 从 `clientStores[clientStoreKey].getClientIdAndIncrement()` 获取 clientId
- 创建 `MWMRQueue.open()` 存入 `reqQueues[reqClientCount++]`
- 返回 clientId

### Requirement: registerResponseClient

`registerResponseClient(int shmKey, int queueSize)` 返回注册的 `MWMRQueue`，对齐 C++ `registerResponseClient()`。

### Requirement: loopMD 轮询

`loopMD()` 对齐 C++ `loopMD()`：遍历 `mdQueues[0..mdClientCount-1]`，非空则 dequeue 并回调 `mdCallback`。

### Requirement: loopMDUntilEndpacket 轮询

`loopMDUntilEndpacket()` 对齐 C++ `loopMD_until_endpacket()`：
1. 调用 `getBestEndPacketClientAsPerTimestampSequencing()` 选择 timestamp 最小的客户端
2. 跳过连续 endPkt（`last_packet_was_endpacket && data.endPkt==1`）
3. 循环 dequeue 直到收到 endPkt=1
4. 回调 `mdCallback`

### Requirement: loopRequest / loopResponse

`loopRequest()` 对齐 C++ `loopRequest()`：遍历 `reqQueues`，非空则 dequeue 并回调 `orsRequestCallback`。
`loopResponse()` 对齐 C++ `loopResponse()`：遍历 `respQueues`，非空则 dequeue 并回调 `orsResponseCallback(data, queueIndex)`。

### Requirement: 线程启停方法

必须对齐以下 C++ 方法：
- `startMonitorAll()` / `startMonitorAsyncAll()` — 组合轮询
- `startMonitorMarketData()` / `startMonitorAsyncMarketData()` — 仅行情
- `startMonitorMarketDataAndResponse()` / `startMonitorAsyncMarketDataAndResponse()` — 行情+回报
- `startMonitorMarketDataAndResponseAndRequest()` — 全部
- `startMonitorORSRequest()` / `startMonitorAsyncORSRequest()` — 请求
- `startMonitorORSRequestHighPerf()` — 高性能请求
- `startMonitorORSResponse()` / `startMonitorAsyncORSResponse()` — 回报
- `shutdown()` — 停止所有线程并 join
- `stopMonitor()` — 设置所有 active 标志为 false
- `waitForCompletion*()` — 等待各线程结束

### Requirement: 回调接口

定义三个回调接口（对齐 C++ DEFINE_SIGNAL 宏）：
- `MDCallback`: `void onMarketUpdate(MemorySegment data)`
- `ORSRequestCallback`: `void onRequest(MemorySegment data)`
- `ORSResponseCallback`: `void onResponse(MemorySegment data, int queueIndex)`
