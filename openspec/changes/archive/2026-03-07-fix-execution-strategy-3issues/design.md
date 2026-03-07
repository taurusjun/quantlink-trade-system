## Context

ExecutionStrategy.java 是所有策略的基类，从 C++ ExecutionStrategy.h/cpp 迁移而来。在对齐验证中发现 3 处缺失：撤单拒绝暂停机制、自营订单簿缓存删除 Map、handleSquareON 基类方法。代码修复已完成并编译通过，现需补齐单元测试。

## Goals / Non-Goals

**Goals:**
- 为 3 项已完成的 C++ 对齐修复编写单元测试
- 测试用例覆盖核心逻辑分支和边界条件
- 确保测试可独立运行，不依赖 SHM 或外部进程

**Non-Goals:**
- 不修改已完成的代码修复（代码已编译通过）
- 不编写集成测试或 E2E 测试
- 不测试 SelfBook/CommonBook 的完整行情处理链路（那是独立功能）

## Decisions

### 1. 测试框架: JUnit 5

项目已使用 JUnit 5，测试文件放在 `tbsrc-java/src/test/java/com/quantlink/trader/strategy/`。

### 2. 使用具体子类测试抽象基类

ExecutionStrategy 是 abstract class，测试需要一个最小具体子类 `TestableExecutionStrategy`，仅实现 `sendOrder()` 空方法。这样可以直接测试基类的 `sendCancelOrder()`、`processCancelReject()`、`removeOrder()`、`handleSquareON()` 等方法。

### 3. Mock 策略

- **Watch**: 使用真实单例，因为 `Watch.getInstance().getCurrentTime()` 返回 `System.nanoTime()`，测试中可直接使用
- **CommonClient**: sendCancelOrder 需要 client 实例，使用 mock 或 stub
- **MemorySegment**: processCancelReject 接受 MemorySegment 参数，需要构造 ResponseMsg 格式的 off-heap buffer

### 4. 测试文件命名

`ExecutionStrategy3IssuesTest.java` — 专门测试这 3 项修复，与现有 ExecutionStrategyTest.java 分离避免冲突。

## Risks / Trade-offs

- [Risk] MemorySegment 构造需要正确的 ResponseMsg 布局偏移 → 使用 Types 类中的 VarHandle 和 RESPONSE_MSG_LAYOUT 确保一致
- [Risk] Watch 时间精度在测试中不可控 → CANCELREQ_PAUSE 测试使用足够大的间隔（或设为 0）避免 flaky test
