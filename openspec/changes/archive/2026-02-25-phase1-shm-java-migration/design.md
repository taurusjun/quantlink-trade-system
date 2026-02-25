## Context

当前系统中，C++ 网关（`md_shm_feeder`、`counter_bridge`）通过 SysV MWMR 共享内存与策略进程通信。C++ 原代码位于 hftbase 库中，包含 SHM 分配、MWMR Queue、消息结构体定义和 Connector 接口。

Java 迁移的 Phase 1 目标是用 JDK 22+ Panama FFI 直接翻译 C++ hftbase 的 SHM/IPC 层，使 Java 策略进程能与 C++ 网关通过 SHM 互操作。

**约束条件**：
- 必须与 C++ hftbase 的 MWMR Queue 二进制兼容（SHM 内存布局逐字节一致）
- 必须在 Linux（生产）和 macOS（开发）上运行
- C++ 网关完全不修改
- 不依赖 JNI/JNA，全部通过 Panama FFI 调用系统 API

## Goals / Non-Goals

**Goals:**

- 用 Panama FFI 封装 SysV SHM 系统调用（`shmget/shmat/shmdt/shmctl`）
- 用 `StructLayout` + `VarHandle` 精确定义三个核心消息结构体的内存布局
- 实现 MWMR Queue 的 lock-free 入队/出队，原子操作语义与 C++ 完全一致
- 实现 Connector：三队列轮询、OrderID 生成、ORS 回报过滤
- 通过 offset 验证测试确保二进制兼容性

**Non-Goals:**

- 策略层翻译（Phase 2-3 范围）
- CommonClient 回调分发（Phase 2 范围）
- 配置加载框架（Phase 4 范围）
- GraalVM native-image 优化（后续优化）
- WebSocket/REST 监控接口（后续 Phase）

## Decisions

### Decision 1: Panama FFI vs JNI vs JNA

**选择**: Panama FFI (JDK 22+ `java.lang.foreign`)

**备选方案**:
- **JNI**: 需要编写 C 胶水代码 + 编译 `.so`/`.dylib`，维护成本高
- **JNA**: 基于反射，调用开销 ~100ns（vs Panama ~5-15ns），对 HFT 场景不可接受
- **sun.misc.Unsafe**: 非公开 API，JDK 未来版本可能移除

**理由**: Panama 是 JDK 22 正式稳定的官方 API，纯 Java 调用系统函数，无需本地代码编译，性能接近 JNI，且有 `MemorySegment` 边界检查保证内存安全。

### Decision 2: 结构体表示 — StructLayout vs 手动 offset 常量

**选择**: 混合方案 — `StructLayout` 定义布局 + `VarHandle` 访问器 + 手动 offset 常量作为验证

**备选方案**:
- **纯 StructLayout**: 声明式定义，自动计算 offset，但 padding 必须手动指定（Panama 不自动对齐）
- **纯手动 offset**: 灵活但易出错，缺乏结构化描述
- **代码生成**: 从 C++ 头文件自动生成 Java 布局，但引入工具链复杂度

**理由**: `StructLayout` 提供声明式的结构体描述，`paddingLayout()` 显式标注 padding 位置使布局一目了然，与 C++ 头文件中的字段定义可以逐行对照。同时保留手动 offset 常量在测试中做交叉验证。

### Decision 3: MWMR Queue 原子操作 — VarHandle vs Unsafe

**选择**: `VarHandle` (on `MemorySegment`)

**备选方案**:
- **sun.misc.Unsafe**: `getAndAddLong()`、`getLongVolatile()`，性能最佳但非公开 API
- **VarHandle on byte[]**: 不支持堆外内存

**理由**: Panama `MemorySegment` 原生支持 `VarHandle` 操作，提供 `getAcquire`/`setRelease`/`getAndAdd` 等精确的内存序语义。与 C++ 的 `std::memory_order` 一一对应：

| C++ 原子操作 | Java VarHandle |
|-------------|---------------|
| `head.fetch_add(1, acq_rel)` | `HEAD_VH.getAndAdd(seg, offset, 1L)` |
| `slot->seqNo` load (implicit acquire) | `SEQ_VH.getAcquire(seg, offset)` |
| `slot->seqNo = myHead` (release store) | `SEQ_VH.setRelease(seg, offset, val)` |
| `asm volatile("" ::: "memory")` (compiler barrier) | `VarHandle.storeStoreFence()` |

### Decision 4: 项目构建工具 — Maven vs Gradle

**选择**: Maven

**理由**: 项目依赖极少（仅 JUnit + SnakeYAML），Maven 的 XML 配置虽然冗长但更显式、更稳定。无需 Gradle 的增量编译优势（项目规模小）。

### Decision 5: Java 包结构 — 对齐 C++ hftbase 目录

**选择**: 按 C++ hftbase 模块结构组织 Java 包

```
com.quantlink.trader/
├── shm/                          # ← hftbase/Ipc/ + hftbase/CommonUtils/
│   ├── SysVShm.java              # ← hftbase/Ipc/include/shmallocator.h + SharedMemory
│   ├── MWMRQueue.java            # ← hftbase/Ipc/include/multiwritermultireadershmqueue.h
│   ├── ClientStore.java          # ← hftbase/Ipc/include/locklessshmclientstore.h
│   ├── Types.java                # ← hftbase/CommonUtils/include/marketupdateNew.h + orderresponse.h
│   └── Constants.java            # ← hftbase/CommonUtils/include/orderresponse.h (枚举 + 常量)
└── connector/                    # ← hftbase/Connector/
    └── Connector.java            # ← hftbase/Connector/include/connector.h
```

**理由**: C++ hftbase 中 SHM 相关代码分散在 `Ipc/`（队列/SHM 分配）和 `CommonUtils/`（消息结构体）两个目录。Java 合并为单一 `shm` 包更紧凑，因为这些类型在 Java 中紧密协作（`MWMRQueue` 直接使用 `Types` 中的 `StructLayout`）。`Connector` 独立成包，对应 C++ 的 `hftbase/Connector/`。

### Decision 6: 平台差异处理

**选择**: 通过 Panama `Linker.defaultLookup()` 直接查找 libc 符号

**备选方案**:
- **syscall 号映射**: 运行时检测 OS 选择不同的 syscall 号（macOS vs Linux 不同）
- **编译时分离**: 需要条件编译或 ServiceLoader，增加复杂度

**理由**: Panama `Linker.defaultLookup()` 可以直接查找 libc 中的 `shmget` / `shmat` 等函数符号（它们在 macOS 和 Linux 的 libc 中都有导出），无需知道 syscall 号。这是最简洁的跨平台方案。C++ hftbase 直接调用 libc 函数（非 raw syscall），Java 方案与 C++ 原代码的调用方式一致。

### Decision 7: QueueElem 大小处理

**选择**: 硬编码 `REQ_QUEUE_ELEM_SIZE = 320`，其他类型自动计算

**理由**: C++ `RequestMsg` 有 `__attribute__((aligned(64)))` 属性（参考 `hftbase/CommonUtils/include/orderresponse.h`），使 `QueueElem<RequestMsg>` 从 264 bytes (256+8) 向上对齐到 320 bytes (5×64)。这是 C++ 编译器行为，Java 的 `StructLayout` 不会自动处理。必须硬编码并在测试中严格验证。

`QueueElemMD`（824 = 816+8）和 `QueueElemResp`（184 = 176+8）无对齐问题，可由 `dataSize + 8` 自动计算。

### Decision 8: MemorySegment 生命周期管理

**选择**: SHM 段使用 `Arena.global()`（进程级生命周期），不受 GC 影响

**备选方案**:
- **Arena.ofConfined()**: 单线程，关闭时自动 detach，但限制了跨线程共享
- **Arena.ofShared()**: 可跨线程，关闭时自动 detach，但 SHM 段应与进程同生命周期

**理由**: C++ hftbase 中 SHM 段在进程启动时 `shmat`、进程退出时由 OS 自动清理（或显式 `shmdt`）。Java 使用 `Arena.global()` 匹配这一生命周期。显式提供 `close()` 方法调用 `shmdt` 用于优雅关闭，对应 C++ `ShmAllocator::cleanup()`。

## Risks / Trade-offs

### Risk 1: StructLayout padding 错误导致数据损坏

**严重度**: 高

**说明**: Panama `StructLayout` 不自动添加 padding。如果 `paddingLayout()` 位置或大小错误，后续所有字段 offset 都会偏移，读写 SHM 时数据损坏。

**缓解**: 根据 C++ 头文件中的字段定义逐字段对照编写 StructLayout，编写全量 offset 断言测试（50+ 个断言），覆盖每个结构体的每个字段。**Phase 1 的硬性门槛是 offset 测试 100% 通过**。

### Risk 2: VarHandle 内存序语义与 C++ 不完全一致

**严重度**: 中

**说明**: Java `VarHandle` 的 `getAcquire`/`setRelease` 在 x86 上等价于 C++ 的 `acquire`/`release`，但在 ARM 上可能有差异（本项目仅运行在 x86 Linux，风险低）。

**缓解**: 生产环境限定 x86-64 Linux。测试中用多线程并发读写验证 MWMR Queue 的正确性。

### Risk 3: Panama API 在 JDK 后续版本中变更

**严重度**: 低

**说明**: Panama FFI 在 JDK 22 正式稳定（JEP 454），后续版本不太可能有破坏性变更。

**缓解**: 锁定 JDK 22+（或 JDK 25 LTS），SHM 层代码量小（~500 行），即使 API 变更迁移成本低。

### Risk 4: macOS 开发环境 SHM 限制

**严重度**: 低

**说明**: macOS 的 SysV SHM 有 segment 数量和大小限制（默认 `kern.sysv.shmmax = 4MB`）。

**缓解**: 开发时使用较小的 queue size。测试脚本在运行前检查/调整 sysctl 参数。生产部署仅在 Linux。

### Trade-off: 代码量 vs 类型安全

Panama `StructLayout` 定义消息结构体比 C++ 头文件中的 struct 定义更冗长（每个字段需声明类型 + 名称 + 显式 padding），但提供了运行时的类型安全和边界检查。接受增加约 30% 的定义代码量换取更好的可维护性和安全性。
