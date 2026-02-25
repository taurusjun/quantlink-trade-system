## Why

Go 翻译 C++ 策略层时遇到了继承体系映射难题 — C++ `ExecutionStrategy` 有 42 个子类，Go 的组合模式 workaround（`ORSCallbackOverride` 等）导致架构变形、维护成本随子类数量线性增长。评估后决定用 Java（JDK 22+ Panama FFI）重写策略层，Java 的 `abstract class` / `extends` / `@Override` 与 C++ 继承体系 1:1 映射。

Phase 1 是整个 Java 迁移的基础层：实现 SysV 共享内存封装、MWMR 队列、消息结构体和 Connector，确保 Java 进程能与现有 C++ 网关（`md_shm_feeder`、`counter_bridge`）通过 SHM 互操作。

## What Changes

- **新建 `tbsrc-java/` Maven 项目**：JDK 22+，使用 Panama Foreign Function & Memory API
- **新建 `shm/SysVShm.java`**：封装 `shmget/shmat/shmdt/shmctl` 系统调用（Panama Linker downcall），支持 Linux/macOS
- **新建 `shm/MWMRQueue.java`**：MWMR 共享内存环形队列，使用 `VarHandle` 原子操作（`getAndAdd`、`getAcquire`、`setRelease`），与 C++ `MultiWriterMultiReaderShmQueue` 二进制兼容
- **新建 `shm/Types.java`**：三个核心消息结构体的 `StructLayout` 定义：
  - `MarketUpdateNew`（816 bytes）— 行情数据
  - `RequestMsg`（256 bytes, aligned(64)）— 订单请求，QueueElem 为 320 bytes
  - `ResponseMsg`（176 bytes）— 订单回报
  - 以及 `ContractDescription`（96 bytes）、`BookElement`（16 bytes）、`MDHeaderPart`（96 bytes）、`MDDataPart`（720 bytes）
- **新建 `shm/Constants.java`**：枚举常量（`RequestType`、`ResponseType`、`OrderType` 等 11 种枚举）和关键常量（`ORDERID_RANGE=1_000_000`、`InterestLevels=20`、`MaxORSClients=250` 等）
- **新建 `shm/ClientStore.java`**：原子 clientId 分配器（16 bytes SHM：`atomic<int64> data` + `int64 firstClientId`）
- **新建 `connector/Connector.java`**：三队列 SHM 轮询 + 发单/撤单，OrderID 生成（`clientId × 1_000_000 + seq`），ORS 回报过滤（`OrderID / ORDERID_RANGE == clientID`）
- **新建完整的 offset 验证测试**：从 Go `types_test.go` 移植全部 offset 断言（BookElement、ContractDescription、RequestMsg、ResponseMsg、MDHeaderPart、MDDataPart、MarketUpdateNew、QueueElem 尺寸），作为二进制兼容性的硬性门槛

## Capabilities

### New Capabilities

- `java-sysv-shm`: SysV 共享内存系统调用封装（Panama Linker downcall: shmget/shmat/shmdt/shmctl），支持 Linux 和 macOS 平台
- `java-mwmr-queue`: MWMR 共享内存环形队列实现（VarHandle 原子操作），与 C++ hftbase `MultiWriterMultiReaderShmQueue` 二进制兼容
- `java-shm-types`: 三个核心消息结构体（MarketUpdateNew/RequestMsg/ResponseMsg）的 Panama StructLayout 定义，含全部枚举常量
- `java-client-store`: 原子 clientId 分配器，与 C++ `LocklessShmClientStore` 二进制兼容
- `java-connector`: SHM 三队列轮询 + 发单/撤单/改单，OrderID 生成和 ORS 回报过滤

### Modified Capabilities

（无修改，全部为新建能力）

## Impact

### 代码影响

| 范围 | 说明 |
|------|------|
| **新增目录** | `tbsrc-java/` — 完整 Maven 项目骨架 |
| **新增源文件** | `shm/` 包 5-6 个 Java 文件 + `connector/` 包 1 个 Java 文件 |
| **新增测试文件** | `shm/` 包 3-4 个测试文件 + `connector/` 包 1 个测试文件 |
| **预估代码量** | 源文件 ~1,000 行 + 测试 ~600 行（参考 Go 实现 1,142 + 686 行） |

### 依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| JDK | 22+ | Panama FFI (JEP 454) 稳定版 |
| JUnit Jupiter | 5.10+ | 单元测试 |
| SnakeYAML | 2.2+ | YAML 配置解析（后续 Phase 使用，此阶段可选） |

### 外部系统

- **不影响** C++ 网关（`md_shm_feeder`、`counter_bridge`）— 完全不修改
- **不影响** Go 策略层（`tbsrc-golang/`）— 保留不动
- **共享** SysV SHM 段（0x1001/0x2001/0x3001/0x4001）— Java 必须与 C++ 二进制兼容

### 关键风险

| 风险 | 缓解措施 |
|------|---------|
| struct 对齐错误导致 SHM 数据损坏 | 从 Go `types_test.go` 移植全部 offset 断言，Phase 1 必须 100% 通过 |
| Panama API 平台差异（macOS vs Linux） | 平台常量隔离（如 Go 的 `sysv_darwin.go` / `sysv_linux.go`） |
| `QueueElem<RequestMsg>` 320 bytes 对齐陷阱 | 硬编码 `REQ_QUEUE_ELEM_SIZE = 320`，测试验证 |

### C++ 原代码参考

| C++ 文件 | Java 目标 |
|----------|----------|
| `hftbase/Ipc/include/multiwritermultireadershmqueue.h` | `shm/MWMRQueue.java` |
| `hftbase/Ipc/include/shmallocator.h` | `shm/SysVShm.java` 的一部分 |
| `hftbase/Ipc/include/locklessshmclientstore.h` | `shm/ClientStore.java` |
| `hftbase/CommonUtils/include/marketupdateNew.h` | `shm/Types.java` |
| `hftbase/CommonUtils/include/orderresponse.h` | `shm/Types.java` + `shm/Constants.java` |
| `hftbase/Connector/include/connector.h` | `connector/Connector.java` |
