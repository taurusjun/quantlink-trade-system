## Why

Java Connector 违反了 C++ 1:1 对齐原则：C++ 中 `Connector` 持有 `ShmMgr m_shmMgr`（即 `MultiClientStoreShmReader<MD,REQ,RESP,MAXSIZE>`），该类负责 SHM 队列注册、ClientStore 管理和轮询线程。Java 将这些职责散落在 `Connector` 类中，缺少对应的 `MultiClientStoreShmReader` 类。

## What Changes

- 新建 `MultiClientStoreShmReader.java`，1:1 对齐 C++ `hftbase/Ipc/include/multiclientstoreshmreader.h`
- 包含：MD/REQ/RESP 队列数组管理、ClientStore 映射、轮询方法（loopMD/loopRequest/loopResponse）、线程启停
- **阶段 1（本次）**: 创建新类，不修改 Connector
- **阶段 2（后续）**: 重构 Connector 使用 MultiClientStoreShmReader

## Capabilities

### New Capabilities
- `java-multi-client-store-shm-reader`: MultiClientStoreShmReader SHM 管理器，对齐 C++ hftbase/Ipc/include/multiclientstoreshmreader.h

### Modified Capabilities

（本次不修改现有 capability）

## Impact

- 新增文件: `tbsrc-java/src/main/java/com/quantlink/trader/shm/MultiClientStoreShmReader.java`
- 不影响现有 Connector 功能（阶段 1 仅新增，不修改）
