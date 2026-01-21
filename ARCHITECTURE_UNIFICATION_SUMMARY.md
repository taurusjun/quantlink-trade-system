# 架构统一完成总结

**日期：** 2026-01-20
**任务：** 统一 MD Gateway 和 ORS Gateway 的架构设计

---

## 🎯 重构目标

将 ORS Gateway 的架构统一到 MD Gateway 的职责分离模式：
- **数据源层**（main_*.cpp）：负责共享内存管理和数据转换
- **服务层**（*_gateway.cpp）：负责业务逻辑和对外服务

---

## 📊 重构前后对比

### 重构前架构（Week 5-6 原始实现）

```
┌─────────────────────────────────────────────────────┐
│ main_ors.cpp (主程序)                               │
│  ├─ 信号处理                                        │
│  ├─ 命令行解析                                      │
│  ├─ Gateway初始化                                   │
│  └─ gRPC服务器启动                                  │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ ors_gateway.cpp (业务逻辑 + 数据源)                │
│  ├─ 共享内存管理 (Create/Open/Close) ← 混合        │
│  ├─ 请求队列写入 ← 混合                            │
│  ├─ 响应队列读取 ← 混合                            │
│  ├─ gRPC服务 (SendOrder)                           │
│  ├─ NATS发布 (PublishOrderUpdate)                  │
│  ├─ 订单簿管理 (UpdateOrderBook)                   │
│  └─ 风控检查 (CheckRisk)                           │
└─────────────────────────────────────────────────────┘
```

**问题：**
- ❌ 职责混乱：数据源和业务逻辑耦合
- ❌ 难以测试：无法Mock共享内存
- ❌ 架构不一致：与MD Gateway风格不同

### 重构后架构（统一架构）

```
┌─────────────────────────────────────────────────────┐
│ main_ors.cpp (数据源层)                             │
│  ├─ 共享内存管理 (Create/Open/Close)               │
│  ├─ 请求队列写入线程 (GetOrderRequest → Push)     │
│  ├─ 响应队列读取线程 (Pop → OnOrderResponse)      │
│  ├─ 数据转换 (Raw ↔ Protobuf)                     │
│  └─ 调用Gateway接口                                 │
└─────────────────┬───────────────────────────────────┘
                  │ 接口调用
                  ▼
┌─────────────────────────────────────────────────────┐
│ ors_gateway.cpp (服务层)                            │
│  ├─ gRPC服务 (SendOrder/CancelOrder/Query)        │
│  ├─ NATS发布 (PublishOrderUpdate)                  │
│  ├─ 订单簿管理 (UpdateOrderBook)                   │
│  ├─ 风控检查 (CheckRisk)                           │
│  ├─ 内部队列 (m_pending_requests)                  │
│  └─ 对外接口 (GetOrderRequest/OnOrderResponse)     │
└─────────────────────────────────────────────────────┘
```

**改进：**
- ✅ 职责清晰：数据源 vs 业务逻辑分离
- ✅ 易于测试：可以Mock数据源
- ✅ 架构统一：与MD Gateway完全一致

---

## 🔧 重构细节

### 1. ors_gateway.h 修改

**移除成员：**
```cpp
// 移除共享内存队列成员
OrderReqQueue* m_request_queue;
OrderRespQueue* m_response_queue;
std::thread m_response_thread;

// 移除队列名称
std::string m_req_queue_name;
std::string m_resp_queue_name;
std::string m_grpc_address;
```

**新增成员：**
```cpp
// 新增内部队列（缓冲待发送订单）
std::queue<OrderRequestRaw> m_pending_requests;
std::mutex m_pending_requests_mutex;
```

**新增接口方法：**
```cpp
// 供main_ors.cpp调用的外部接口
bool GetOrderRequest(OrderRequestRaw* raw_req);  // 获取待发送订单
void OnOrderResponse(const OrderUpdate& update); // 处理订单回报
```

**移除方法：**
```cpp
void ProcessResponseQueueThread();  // 不再由Gateway管理线程
```

### 2. ors_gateway.cpp 修改

**Initialize() 简化：**
```cpp
// 移除前（67-78行）：
auto* req_queue_raw = hft::shm::ShmManager::Create(m_req_queue_name);
m_request_queue = reinterpret_cast<OrderReqQueue*>(req_queue_raw);
auto* resp_queue_raw = hft::shm::ShmManager::Open(m_resp_queue_name);
m_response_queue = reinterpret_cast<OrderRespQueue*>(resp_queue_raw);

// 移除后：
// 只保留NATS初始化，不再管理共享内存
```

**SendOrder() 修改：**
```cpp
// 修改前（176行）：
if (!m_request_queue->Push(raw_req)) { ... }  // 直接写共享内存

// 修改后（135-139行）：
{
    std::lock_guard<std::mutex> lock(m_pending_requests_mutex);
    m_pending_requests.push(raw_req);  // 写入内部队列
}
```

**新增方法实现：**
```cpp
// GetOrderRequest() - 供main_ors.cpp获取待发送订单
bool ORSGatewayImpl::GetOrderRequest(OrderRequestRaw* raw_req) {
    std::lock_guard<std::mutex> lock(m_pending_requests_mutex);
    if (m_pending_requests.empty()) return false;
    *raw_req = m_pending_requests.front();
    m_pending_requests.pop();
    return true;
}

// OnOrderResponse() - 供main_ors.cpp推送订单回报
void ORSGatewayImpl::OnOrderResponse(const OrderUpdate& update) {
    UpdateOrderBook(update);
    #ifdef ENABLE_NATS
    PublishOrderUpdate(update);
    #endif
}
```

**移除内容：**
- 删除 `ProcessResponseQueueThread()` 方法（314-348行）
- 删除 `Start()` 中的线程启动代码
- 删除 `Stop()` 中的共享内存清理代码
- 删除 `ConvertToProto()` 方法（移到匿名命名空间）

### 3. main_ors.cpp 重构

**新增头文件：**
```cpp
#include "shm_queue.h"
#include <thread>
#include <atomic>
```

**新增类型定义：**
```cpp
using OrderReqQueue = hft::shm::SPSCQueue<hft::ors::OrderRequestRaw, 4096>;
using OrderRespQueue = hft::shm::SPSCQueue<hft::ors::OrderResponseRaw, 4096>;
```

**新增全局变量：**
```cpp
static std::atomic<bool> g_running{true};  // 控制队列线程
```

**新增转换函数：**
```cpp
void ConvertToProtobuf(const hft::ors::OrderResponseRaw& raw_resp,
                       hft::ors::OrderUpdate* proto_update) {
    // 将原始数据转换为Protobuf格式
    proto_update->set_order_id(raw_resp.order_id);
    proto_update->set_client_order_id(raw_resp.client_order_id);
    // ... 其他字段转换
}
```

**新增队列线程：**

1. **请求队列写入线程**（74-112行）：
```cpp
void RequestQueueWriterThread(ORSGatewayImpl* gateway, OrderReqQueue* req_queue) {
    while (g_running.load()) {
        OrderRequestRaw raw_req;
        if (gateway->GetOrderRequest(&raw_req)) {
            if (req_queue->Push(raw_req)) {
                written_count++;
                // 定期打印统计
            }
        } else {
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }
}
```

2. **响应队列读取线程**（115-154行）：
```cpp
void ResponseQueueReaderThread(ORSGatewayImpl* gateway, OrderRespQueue* resp_queue) {
    while (g_running.load()) {
        OrderResponseRaw raw_resp;
        if (resp_queue->Pop(raw_resp)) {
            OrderUpdate proto_update;
            ConvertToProtobuf(raw_resp, &proto_update);
            gateway->OnOrderResponse(proto_update);  // 调用Gateway接口
            read_count++;
        } else {
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }
}
```

**main() 函数流程**（现在有10个步骤）：
```cpp
int main(int argc, char** argv) {
    // 1. 创建/打开共享内存队列
    auto* req_queue_raw = hft::shm::ShmManager::Create(req_queue_name);
    auto* req_queue = reinterpret_cast<OrderReqQueue*>(req_queue_raw);

    auto* resp_queue_raw = hft::shm::ShmManager::Open(resp_queue_name);
    auto* resp_queue = reinterpret_cast<OrderRespQueue*>(resp_queue_raw);

    // 2. 创建ORS Gateway实例
    auto gateway = std::make_unique<ORSGatewayImpl>();

    // 3. 初始化Gateway
    gateway->Initialize(config_file);

    // 4. 启动Gateway
    gateway->Start();

    // 5. 启动请求队列写入线程
    std::thread req_writer_thread([&]() { ... });

    // 6. 启动响应队列读取线程
    std::thread resp_reader_thread([&]() { ... });

    // 7. 构建gRPC服务器
    grpc::ServerBuilder builder;
    builder.AddListeningPort(grpc_address, ...);
    builder.RegisterService(gateway.get());

    // 8. 启动gRPC服务器
    g_server = builder.BuildAndStart();

    // 9. 等待关闭信号
    g_server->Wait();

    // 10. 清理（停止线程、关闭队列、打印统计）
    g_running = false;
    req_writer_thread.join();
    resp_reader_thread.join();
    gateway->Stop();
    munmap(req_queue_raw, sizeof(OrderReqQueue));
    munmap(resp_queue_raw, sizeof(OrderRespQueue));
}
```

---

## 📈 架构对比总结

| 特性 | 重构前 | 重构后 |
|------|--------|--------|
| **共享内存管理位置** | ors_gateway.cpp | main_ors.cpp |
| **队列读写线程** | Gateway内部线程 | main独立线程 |
| **数据转换位置** | Gateway内部 | main独立函数 |
| **Gateway职责** | 数据源+业务逻辑 | 纯业务逻辑 |
| **可测试性** | 难以Mock | 易于Mock |
| **与MD Gateway一致性** | ❌ 不一致 | ✅ 完全一致 |
| **代码行数** | ors_gateway.cpp: 513行 | ors_gateway.cpp: 452行 |
|  | main_ors.cpp: 127行 | main_ors.cpp: 307行 |

---

## 🎯 架构原则（现在统一遵循）

### 1. 单一职责原则（SRP）

**数据源层（main_*.cpp）：**
- ✅ 共享内存生命周期管理
- ✅ 队列读写操作
- ✅ 数据格式转换（Raw ↔ Protobuf）
- ✅ 调用Gateway接口

**服务层（*_gateway.cpp）：**
- ✅ gRPC/NATS等对外服务
- ✅ 业务逻辑处理
- ✅ 状态管理（订单簿、统计）
- ❌ 不涉及数据源细节

### 2. 依赖倒置原则（DIP）

```cpp
// Gateway不依赖具体的数据源实现
class ORSGatewayImpl {
    // 对外接口（由数据源调用）
    bool GetOrderRequest(OrderRequestRaw* raw_req);
    void OnOrderResponse(const OrderUpdate& update);
};

// main_ors.cpp 依赖Gateway接口
void RequestQueueWriterThread(ORSGatewayImpl* gateway, ...) {
    gateway->GetOrderRequest(&raw_req);  // 通过接口调用
}

void ResponseQueueReaderThread(ORSGatewayImpl* gateway, ...) {
    gateway->OnOrderResponse(proto_update);  // 通过接口调用
}
```

### 3. 开闭原则（OCP）

**扩展性示例：**
- 添加TCP数据源：只需新增 `main_ors_tcp.cpp`
- 添加WebSocket数据源：只需新增 `main_ors_ws.cpp`
- Gateway代码无需改动

---

## 🔍 数据流对比

### 重构前：订单请求流程

```
Client gRPC Call
    ↓
ORSGateway::SendOrder()
    ↓ (直接写)
共享内存请求队列 ← ❌ 耦合
    ↓
Counter Gateway
```

### 重构后：订单请求流程

```
Client gRPC Call
    ↓
ORSGateway::SendOrder()
    ↓ (写内部队列)
m_pending_requests (内部缓冲)
    ↓ (GetOrderRequest接口)
main_ors.cpp 请求队列写入线程
    ↓ (Push)
共享内存请求队列 ✅ 解耦
    ↓
Counter Gateway
```

### 重构前：订单回报流程

```
Counter Gateway
    ↓
共享内存响应队列
    ↓ (ProcessResponseQueueThread内部读取)
ORSGateway::ProcessResponseQueueThread() ← ❌ 耦合
    ↓
UpdateOrderBook() + PublishOrderUpdate()
```

### 重构后：订单回报流程

```
Counter Gateway
    ↓
共享内存响应队列
    ↓ (Pop)
main_ors.cpp 响应队列读取线程 ✅ 解耦
    ↓ (ConvertToProtobuf)
Protobuf格式
    ↓ (OnOrderResponse接口)
ORSGateway::OnOrderResponse()
    ↓
UpdateOrderBook() + PublishOrderUpdate()
```

---

## ✅ 验证结果

### 编译验证

```bash
$ cmake --build . --target ors_gateway
[  8%] Building CXX object CMakeFiles/ors_gateway.dir/src/main_ors.cpp.o
[ 16%] Building CXX object CMakeFiles/ors_gateway.dir/src/ors_gateway.cpp.o
[ 25%] Linking CXX executable ors_gateway
[100%] Built target ors_gateway

$ ls -lh ors_gateway
-rwxr-xr-x  1 user  staff   831K Jan 20 16:35 ors_gateway
```

**编译结果：**
- ✅ 编译成功
- ✅ 可执行文件大小：831 KB
- ⚠️  5个警告（unused parameter，无害）

### 架构一致性验证

对比 MD Gateway 和 ORS Gateway 的架构：

| 组件 | MD Gateway | ORS Gateway | 一致性 |
|------|-----------|-------------|--------|
| **共享内存管理** | main_md.cpp | main_ors.cpp | ✅ |
| **队列读取线程** | SharedMemoryReaderThread | ResponseQueueReaderThread | ✅ |
| **队列写入线程** | N/A（只读） | RequestQueueWriterThread | ✅ |
| **数据转换函数** | ConvertToProtobuf | ConvertToProtobuf | ✅ |
| **Gateway接口** | PushMarketData | GetOrderRequest/OnOrderResponse | ✅ |
| **Gateway职责** | 纯业务逻辑 | 纯业务逻辑 | ✅ |

---

## 📝 代码统计

### 修改文件清单

| 文件 | 修改类型 | 改动行数 |
|------|---------|---------|
| `ors_gateway.h` | 重构 | +15 / -25 |
| `ors_gateway.cpp` | 重构 | +35 / -96 |
| `main_ors.cpp` | 重写 | +180 / -0 |
| **总计** | - | **+230 / -121** |

### 代码结构对比

**重构前：**
- ors_gateway.h: 194行
- ors_gateway.cpp: 513行
- main_ors.cpp: 127行
- **总计：834行**

**重构后：**
- ors_gateway.h: 184行 ✅ (-10行)
- ors_gateway.cpp: 452行 ✅ (-61行)
- main_ors.cpp: 307行 ⚠️ (+180行)
- **总计：943行** (+109行)

**分析：**
- Gateway代码减少了71行（更纯粹的业务逻辑）
- main增加了180行（承担了数据源职责）
- 总代码增加是合理的（职责分离带来的必要复杂度）

---

## 🎨 设计模式应用

### 1. 关注点分离（Separation of Concerns）

```
数据源层（main_*.cpp）          服务层（*_gateway.cpp）
━━━━━━━━━━━━━━━━━━━━          ━━━━━━━━━━━━━━━━━━━━━
- 共享内存管理                  - gRPC服务
- 队列读写                      - NATS发布
- 数据转换                      - 业务逻辑
- 线程管理                      - 状态管理
```

### 2. 接口抽象（Interface Abstraction）

```cpp
// Gateway提供抽象接口，不暴露内部实现
class ORSGatewayImpl {
public:
    // 外部调用接口（面向main）
    bool GetOrderRequest(OrderRequestRaw* raw_req);
    void OnOrderResponse(const OrderUpdate& update);

    // gRPC服务接口（面向客户端）
    grpc::Status SendOrder(...);
    grpc::Status CancelOrder(...);

private:
    // 内部实现细节
    std::queue<OrderRequestRaw> m_pending_requests;
    std::unordered_map<std::string, OrderInfo> m_orders;
};
```

### 3. 生产者-消费者模式

```
           请求流
┌────────────────────────────────────────┐
│  gRPC Client                           │
│    ↓ (生产订单请求)                     │
│  ORSGateway::SendOrder()              │
│    ↓ (推送到内部队列)                   │
│  m_pending_requests                   │
│    ↓ (消费订单请求)                     │
│  RequestQueueWriterThread             │
│    ↓ (写入共享内存)                     │
│  Shared Memory Request Queue          │
└────────────────────────────────────────┘

           响应流
┌────────────────────────────────────────┐
│  Shared Memory Response Queue         │
│    ↓ (读取订单回报)                     │
│  ResponseQueueReaderThread            │
│    ↓ (数据转换)                        │
│  ConvertToProtobuf()                  │
│    ↓ (推送回报)                        │
│  ORSGateway::OnOrderResponse()        │
│    ↓ (更新状态+发布)                    │
│  UpdateOrderBook() + NATS Publish     │
└────────────────────────────────────────┘
```

---

## 💡 架构优势

### 1. 可测试性

**Mock数据源示例：**
```cpp
// 测试时可以轻松Mock数据源
class MockOrderDataSource {
public:
    void SimulateOrderResponse(const OrderUpdate& update) {
        gateway->OnOrderResponse(update);  // 直接调用接口
    }
};

// 单元测试
TEST(ORSGateway, HandleOrderResponse) {
    auto gateway = std::make_unique<ORSGatewayImpl>();
    MockOrderDataSource mock;

    OrderUpdate update;
    update.set_order_id("TEST_123");
    update.set_status(OrderStatus::FILLED);

    mock.SimulateOrderResponse(update);

    // 验证订单簿是否正确更新
    EXPECT_EQ(gateway->GetStatistics().filled_orders, 1);
}
```

### 2. 灵活性

**支持多种数据源：**
```cpp
// 共享内存数据源
./main_ors --req-queue ors_request --resp-queue ors_response

// TCP数据源（未来）
./main_ors_tcp --server tcp://localhost:9000

// WebSocket数据源（未来）
./main_ors_ws --server ws://localhost:8080
```

### 3. 可维护性

**职责清晰：**
- 修改共享内存格式：只需修改 main_ors.cpp
- 修改订单业务逻辑：只需修改 ors_gateway.cpp
- 两者互不影响

**示例：添加新的订单字段**
```
1. 修改 OrderRequestRaw 结构（ors_gateway.h）
2. 修改 ConvertToProtobuf（main_ors.cpp）
3. ors_gateway.cpp 无需改动 ✅
```

---

## 🚀 下一步建议

### 短期（Week 7-8）

1. **实现 Counter Gateway**
   - 创建 `counter_gateway.cpp`
   - 连接到 ORS Gateway 的共享内存队列
   - 对接 EES/CTP API

2. **创建 Golang 订单客户端**
   - 实现 SendOrder gRPC 客户端
   - 实现 NATS 订单回报订阅
   - 性能测试

### 中期（Week 9-12）

1. **性能优化**
   - 批量读写队列（减少原子操作）
   - CPU亲和性设置
   - 零拷贝优化

2. **监控和告警**
   - Prometheus指标集成
   - 队列利用率监控
   - 延迟P99告警

### 长期（Week 13+）

1. **高可用架构**
   - 主备Gateway切换
   - 订单持久化
   - 故障自动恢复

2. **多数据源支持**
   - 抽象数据源接口
   - TCP/WebSocket数据源
   - 动态数据源切换

---

## 📚 参考文档

- [ARCHITECTURE_INCONSISTENCY_ANALYSIS.md](ARCHITECTURE_INCONSISTENCY_ANALYSIS.md) - 架构差异详细分析
- [SHM_EXAMPLE.md](SHM_EXAMPLE.md) - 共享内存使用示例
- [WEEK56_ORS_GATEWAY_SUMMARY.md](WEEK56_ORS_GATEWAY_SUMMARY.md) - Week 5-6 实现总结
- [UNIFIED_ARCHITECTURE_DESIGN.md](/Users/user/PWorks/RD/docs/hftbase/unified_architecture_design.md) - 统一架构设计

---

## ✅ 结论

**架构统一已完成！**

- ✅ MD Gateway 和 ORS Gateway 现在采用相同的架构模式
- ✅ 职责分离：数据源层 vs 服务层
- ✅ 代码可维护性和可测试性显著提升
- ✅ 为后续扩展（Counter Gateway、多数据源）奠定坚实基础

**核心原则：**
> **"数据源与业务逻辑分离，接口抽象与实现解耦"**

**下一步：**
继续按照统一架构设计，实现 Week 7-8 的任务（Counter Gateway + Golang客户端）。

---

**重构完成时间：** 2026-01-20
**编译状态：** ✅ 成功
**架构验证：** ✅ 通过
**代码质量：** ✅ 良好
