## 1. 项目骨架

- [x] 1.1 创建 `tbsrc-java/` Maven 项目：`pom.xml`（JDK 22+, JUnit 5, SnakeYAML），标准 `src/main/java` + `src/test/java` 目录结构
- [x] 1.2 创建包目录：`com/quantlink/trader/shm/`、`com/quantlink/trader/connector/`
- [x] 1.3 验证 Maven 编译通过：`mvn compile` + `mvn test`（空项目）

## 2. SysV SHM 封装

- [x] 2.1 翻译 `SysVShm.java`：Panama Linker downcall 封装 `shmget/shmat/shmdt/shmctl`（迁移自 `hftbase/Ipc/include/shmallocator.h` + `hftbase/Ipc/src/sharedmemory.cpp`）
- [x] 2.2 实现 `ShmSegment` 类：持有 `shmid`、`MemorySegment`、`size`，提供 `open()`/`create()`/`detach()`/`remove()` 方法
- [x] 2.3 实现页对齐逻辑和 `IPC_CREAT`/`IPC_EXCL` 错误回退（create 时 key 已存在回退为 open）
- [x] 2.4 编写 `SysVShmTest.java`：测试 create → 写入 → detach → open → 读取 → destroy 完整流程

## 3. 消息结构体 StructLayout 定义

- [x] 3.1 翻译 `Types.java` — `BookElement` StructLayout（16 bytes），迁移自 `hftbase/CommonUtils/include/marketupdateNew.h: bookElement_t`
- [x] 3.2 翻译 `Types.java` — `ContractDescription` StructLayout（96 bytes），迁移自 `hftbase/CommonUtils/include/orderresponse.h: ContractDescription`
- [x] 3.3 翻译 `Types.java` — `MDHeaderPart` StructLayout（96 bytes）+ `MDDataPart` StructLayout（720 bytes），迁移自 `hftbase/CommonUtils/include/marketupdateNew.h`
- [x] 3.4 翻译 `Types.java` — `MarketUpdateNew` StructLayout（816 bytes = MDHeaderPart + MDDataPart），迁移自 `hftbase/CommonUtils/include/marketupdateNew.h: MarketUpdateNew`
- [x] 3.5 翻译 `Types.java` — `RequestMsg` StructLayout（256 bytes, aligned(64)），迁移自 `hftbase/CommonUtils/include/orderresponse.h: RequestMsg`，特别注意 offset 165 的 20B padding、offset 217 的 3B padding、offset 224 的 32B 尾部 padding
- [x] 3.6 翻译 `Types.java` — `ResponseMsg` StructLayout（176 bytes），迁移自 `hftbase/CommonUtils/include/orderresponse.h: ResponseMsg`，特别注意 offset 20 的 4B padding、offset 102 的 2B padding、offset 167 的 1B padding、offset 172 的 4B 尾部 padding
- [x] 3.7 为每个结构体生成 `VarHandle` 访问器（通过 `StructLayout.varHandle(PathElement.groupElement("fieldName"))`）
- [x] 3.8 翻译 `Constants.java` — 全部 11 种枚举类型 + 关键常量（ORDERID_RANGE、InterestLevels、MaxSymbolSize 等），迁移自 `hftbase/CommonUtils/include/orderresponse.h` 和 `marketupdateNew.h`

## 4. Offset 验证测试（硬性门槛）

- [x] 4.1 编写 `TypesTest.java` — `BookElement` 大小和字段 offset 断言（quantity=0, orderCount=4, price=8, total=16）
- [x] 4.2 编写 `TypesTest.java` — `ContractDescription` 大小和字段 offset 断言（instrumentName=0, symbol=32, expiryDate=84, strikePrice=88, optionType=92, caLevel=94, total=96）
- [x] 4.3 编写 `TypesTest.java` — `MDHeaderPart` 大小和字段 offset 断言（exchTS=0, timestamp=8, seqnum=16, rptSeqnum=24, tokenId=32, symbol=40, symbolID=88, exchangeName=90, total=96）
- [x] 4.4 编写 `TypesTest.java` — `MDDataPart` 大小和字段 offset 断言（newPrice=0, bidUpdates=56, askUpdates=376, validBids=708, feedType=714, total=720）
- [x] 4.5 编写 `TypesTest.java` — `MarketUpdateNew` 大小断言（header=0, data=96, total=816）
- [x] 4.6 编写 `TypesTest.java` — `RequestMsg` 大小和全字段 offset 断言（contractDesc=0, requestType=96, ..., strategyID=220, total=256）
- [x] 4.7 编写 `TypesTest.java` — `ResponseMsg` 大小和全字段 offset 断言（responseType=0, ..., strategyID=168, total=176）
- [x] 4.8 编写 `TypesTest.java` — QueueElem 大小断言：`QueueElemMD=824, QueueElemReq=320, QueueElemResp=184, MWMRHeader=8, ClientData=16`
- [x] 4.9 运行 `mvn test`，全部 offset 断言 MUST 100% 通过后才可进入后续任务

## 5. MWMR Queue 实现

- [x] 5.1 翻译 `MWMRQueue.java` — SHM 内存布局（header + elem 数组），`nextPowerOf2()` 辅助方法，迁移自 `hftbase/Ipc/include/multiwritermultireadershmqueue.h`
- [x] 5.2 实现连接模式构造：`SysVShm.open()` + 读取 head 设置 localTail，迁移自 C++ `init(shmkey, shmsize, flag=0)`
- [x] 5.3 实现创建模式构造：`SysVShm.create()` + 初始化 head=1，迁移自 C++ `init(shmkey, shmsize, flag=IPC_CREAT)`
- [x] 5.4 翻译 `enqueue()` — `VarHandle.getAndAdd` 原子占位 → `MemorySegment.copy` 数据 → `storeStoreFence` → `VarHandle.setRelease` 发布 seqNo，迁移自 C++ `enqueue(const T &value)`
- [x] 5.5 翻译 `dequeue()` — `VarHandle.getAcquire` 检查 seqNo → `MemorySegment.copy` 拷贝数据 → 推进 localTail，迁移自 C++ `dequeuePtr(T* data)`
- [x] 5.6 翻译 `isEmpty()` / `close()` / `destroy()`
- [x] 5.7 支持 elemSize override 参数，用于 RequestMsg 队列传入 320

## 6. MWMR Queue 测试

- [x] 6.1 编写 `MWMRQueueTest.java` — 单线程入队/出队测试（MarketUpdateNew 类型）
- [x] 6.2 编写 `MWMRQueueTest.java` — 单线程入队/出队测试（RequestMsg 类型，elemSize=320）
- [x] 6.3 编写 `MWMRQueueTest.java` — 多线程并发入队 + 单线程出队，验证无数据丢失
- [x] 6.4 编写 `MWMRQueueTest.java` — 空队列出队返回 false
- [x] 6.5 编写 `MWMRQueueTest.java` — 队列环绕测试（写满一圈后继续写入）

## 7. ClientStore 实现

- [x] 7.1 翻译 `ClientStore.java` — SHM 布局（16 bytes），`getClientId()`、`getClientIdAndIncrement()`、`getFirstClientIdValue()`，迁移自 `hftbase/Ipc/include/locklessshmclientstore.h`
- [x] 7.2 实现连接模式和创建模式
- [x] 7.3 编写 `ClientStoreTest.java` — 单线程 increment 测试、多线程并发 increment 唯一性测试

## 8. Connector 实现

- [x] 8.1 翻译 `Connector.java` — 构造函数：连接三队列 + ClientStore，获取 clientId，迁移自 `hftbase/Connector/include/connector.h: Connector`
- [x] 8.2 翻译 `sendNewOrder()` — 设置 NEWORDER + 生成 OrderID + 入队，迁移自 C++ `SendNewOrder(RequestMsg&)`
- [x] 8.3 翻译 `sendCancelOrder()` / `sendModifyOrder()` — 设置 RequestType + 入队，迁移自 C++ `SendCancelOrder` / `SendModifyOrder`
- [x] 8.4 翻译 `nextOrderID()` — `clientId × ORDERID_RANGE + seq.getAndIncrement()`，迁移自 C++ `GetUniqueOrderNumber()`
- [x] 8.5 实现 `start()` / `stop()` — 启动 pollMD + pollORS 两个线程，running 标志控制退出
- [x] 8.6 实现 `pollMD()` — 循环 dequeue MarketUpdateNew，调用 mdCallback
- [x] 8.7 实现 `pollORS()` — 循环 dequeue ResponseMsg，过滤 `OrderID / ORDERID_RANGE == clientId` 后调用 orsCallback

## 9. Connector 测试

- [x] 9.1 编写 `ConnectorTest.java` — OrderID 生成测试（验证 clientId × 1_000_000 + seq 格式）
- [x] 9.2 编写 `ConnectorTest.java` — ORS 过滤测试（属于本 client 的通过，其他 client 的跳过）
- [x] 9.3 编写 `ConnectorTest.java` — 完整流程：入队行情 → mdCallback 触发 → sendNewOrder → 入队回报 → orsCallback 触发

## 10. 跨进程互操作验证

- [x] 10.1 验证 Java 读取 C++ `md_shm_feeder` 写入的 MarketUpdateNew：启动 `md_shm_feeder sim` → Java 连接 SHM 0x1001 → 成功解析 symbol、price 等字段
- [x] 10.2 验证 Java 写入 RequestMsg 被 C++ `counter_bridge` 正确读取：Java 入队到 SHM 0x2001 → counter_bridge 收到订单
- [x] 10.3 验证 Java 读取 C++ `counter_bridge` 写入的 ResponseMsg：counter_bridge 写回报到 SHM 0x3001 → Java 正确解析 OrderID、responseType 等
