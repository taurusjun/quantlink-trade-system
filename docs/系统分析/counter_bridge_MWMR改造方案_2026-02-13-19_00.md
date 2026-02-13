# counter_bridge MWMR 改造方案

**文档日期**: 2026-02-13
**版本**: v1.1
**相关模块**: gateway/src/counter_bridge.cpp
**前置文档**:
- MWMR 技术规格: `docs/系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md`
- 架构更新: `docs/系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md`

---

## 概述

将 counter_bridge 从新 gateway 的 POSIX SPSC 队列 + `OrderRequestRaw`/`OrderResponseRaw` 改造为 hftbase 兼容的 SysV MWMR 队列 + `RequestMsg`/`ResponseMsg`，使 Go trader（tbsrc-golang）可通过共享内存直接对接 counter_bridge。

同时新增 `SetCombOffsetFlag`（开平自动推断）和 `mapContractPos`（持仓跟踪），使 counter_bridge 具备原 ORS 的核心交易管理能力。

**删除 HTTP 持仓查询端点**（`GET /positions`）——该端点是新系统额外添加的，原 C++ 系统中不存在。改造后 Go 通过 MWMR response queue 的 TRADE_CONFIRM 累计跟踪持仓，与原 C++ 策略行为一致。

**ITDPlugin 接口及所有 plugin（CTP、Simulator）零改动。**

---

## 1. 当前架构 vs 改造后架构

### 改造前

```
golang trader → [gRPC] → ors_gateway → [POSIX SPSC SHM] → counter_bridge → ITDPlugin → 交易所
                                        OrderRequestRaw                       ↓
                                        OrderResponseRaw                  CTP / Simulator
```

### 改造后

```
go_trader (tbsrc-golang) ──→ [SysV MWMR SHM, key=REQUEST_SHMKEY] ──→ counter_bridge → ITDPlugin → 交易所
                         ←── [SysV MWMR SHM, key=RESPONSE_SHMKEY] ←──                    ↓
                         ←── [SysV MWMR SHM, key=MD_SHMKEY]       ←── MD feeder      CTP / Simulator
```

---

## 2. HTTP 持仓查询端点移除

### 2.1 背景

当前 counter_bridge 包含一个 HTTP 端点 `GET /positions`（`counter_bridge.cpp:299-408`），Go trader 通过 `ORSClient.QueryPositions()` 调用它获取 CTP 持仓。

**这个端点在原 C++ 系统中不存在。** 原 C++ 系统的持仓模型是：

```
策略端（tbsrc）                          ORS 端
├─ 启动: 读 daily_init.<id> 文件          ├─ 启动: 读 position CSV 文件
├─ 运行: TRADE_CONFIRM 累计计数            ├─ 运行: updatePosition() 独立跟踪
├─ 退出: SaveMatrix2() 写回文件           └─ 退出: writePositionToFile()
└─ 跨策略共享: tcache SHM
```

**关键：策略和 ORS 各自独立跟踪持仓，互不查询。**

### 2.2 当前数据流（改造前）

```
CTP 交易所
    ↓
[counter_bridge]  GET /positions (port 8080)  ← 新系统额外添加
    ↓  HTTP
[Go ORSClient.QueryPositions()]
    ↓
[Trader.positionsByExchange]
    ↓
    ├→ REST API: GET /api/v1/positions          ← 新系统额外添加
    ├→ REST API: GET /api/v1/positions/summary   ← 新系统额外添加
    └→ WebSocket 推送                            ← 新系统额外添加
```

### 2.3 改造后数据流

```
策略持仓（Go trader 内部）:
  ├─ 启动: 读 daily_init 文件（或 position JSON）
  ├─ 运行: MWMR response queue → TRADE_CONFIRM 累计  ← 与原 C++ 一致
  └─ 退出: 写回持仓文件

counter_bridge 持仓（mapContractPos）:
  ├─ 启动: 读 position CSV 文件
  ├─ 运行: SetCombOffsetFlag + updatePosition 独立跟踪  ← 与原 ORS 一致
  └─ 退出: 写回 position CSV

Web 监控（可选，保留在 Go 层）:
  └─ Go trader 直接提供 REST/WS API（从策略内部状态读取，不再依赖 counter_bridge）
```

### 2.4 需要移除的代码

| 位置 | 内容 | 行号 |
|------|------|------|
| `counter_bridge.cpp` | `HandlePositionQuery` 函数 | 299-401 |
| `counter_bridge.cpp` | `g_http_server->Get("/positions", HandlePositionQuery)` 路由注册 | 408 |
| `counter_bridge.cpp` | HTTP server 相关初始化（如不再需要其他端点） | 视情况 |

**Go 端对应移除**（后续 Go 代码改造时处理）:

| 位置 | 内容 |
|------|------|
| `golang/pkg/client/ors_client.go` | `QueryPositions()` 方法（lines 311-383） |
| `golang/pkg/trader/trader.go` | `positionsByExchange` 相关逻辑 |
| `golang/pkg/trader/api.go` | `/api/v1/positions` 端点改为从策略内部状态读取 |

### 2.5 Go 策略持仓跟踪方式变更

| 功能 | 改造前（HTTP 查询） | 改造后（TRADE_CONFIRM 累计） |
|------|-------------------|---------------------------|
| 初始仓位 | HTTP GET counter_bridge/positions | 读 daily_init 文件（与 C++ 一致） |
| 运行时持仓 | 无独立跟踪，依赖初始查询+策略估算 | MWMR response queue TRADE_CONFIRM 累计（与 C++ 一致） |
| Web 监控 | counter_bridge 提供 → Go 代理 | Go 直接从策略状态提供 |
| ORS 开平推断 | 无（gRPC 层处理） | counter_bridge SetCombOffsetFlag 独立跟踪 |

---

## 3. 改动清单

### 3.1 删除内容

| 位置 | 内容 | 原因 |
|------|------|------|
| `counter_bridge.cpp` | `HandlePositionQuery` 函数 + HTTP `/positions` 路由 | 原 C++ 不存在，改造后 Go 不再通过 HTTP 查持仓 |
| `counter_bridge.cpp` | HTTP server（如无其他端点使用） | 不再需要 |

### 3.2 新增文件

| 文件 | 用途 | 行数 |
|------|------|------|
| `gateway/include/hftbase_shm.h` | SysV MWMR 精简实现（二进制兼容 hftbase） | ~200 |

### 3.3 修改文件

| 文件 | 改动内容 | 行数 |
|------|---------|------|
| `counter_bridge.cpp` | 删除 HTTP 端点、SHM 初始化、消息转换、SetCombOffsetFlag、持仓跟踪 | ~390 |

### 3.4 不改动的文件

| 文件 | 原因 |
|------|------|
| `gateway/include/plugin/td_plugin_interface.h` | 插件接口不变 |
| `gateway/plugins/ctp/` | CTP 插件不变 |
| `gateway/plugins/simulator/` | Simulator 插件不变 |
| `gateway/include/shm_queue.h` | 保留，供旧 golang/ 代码使用直到迁移完成 |
| `gateway/include/ors_gateway.h` | 保留，供旧 golang/ 代码使用直到迁移完成 |

---

## 4. hftbase_shm.h — SysV MWMR 精简实现

### 4.1 设计原则

不直接 include hftbase 头文件（依赖链太深），而是写一个**内存布局 100% 兼容**的精简实现。与 Go 的 `pkg/shm/mwmr_queue.go` 是同一份布局的 C++ 版本，共用 offset_check 验证。

### 4.2 包含内容

```cpp
// gateway/include/hftbase_shm.h
#pragma once

#include <atomic>
#include <cstring>
#include <cstdint>
#include <sys/ipc.h>
#include <sys/shm.h>
#include <stdexcept>

namespace hftbase_compat {

// ============================================================
// 1. SysV 共享内存封装
// ============================================================

// 创建 SysV SHM 段（server 端 — counter_bridge 调用）
// C++ 源: hftbase/Ipc/include/sharedmemory.h
inline void* shm_create(int key, size_t size) {
    // 页面对齐
    long page_size = sysconf(_SC_PAGESIZE);
    size = size + page_size - (size % page_size);

    int shmid = shmget(key, size, IPC_CREAT | 0666);
    if (shmid < 0) throw std::runtime_error("shmget create failed");
    void* addr = shmat(shmid, nullptr, 0);
    if (addr == (void*)-1) throw std::runtime_error("shmat failed");
    return addr;
}

// 打开已存在的 SysV SHM 段（client 端）
inline void* shm_open_existing(int key, size_t size) {
    long page_size = sysconf(_SC_PAGESIZE);
    size = size + page_size - (size % page_size);

    int shmid = shmget(key, size, 0666);
    if (shmid < 0) throw std::runtime_error("shmget open failed");
    void* addr = shmat(shmid, nullptr, 0);
    if (addr == (void*)-1) throw std::runtime_error("shmat failed");
    return addr;
}

inline void shm_detach(void* addr) {
    shmdt(addr);
}

// ============================================================
// 2. MWMR Queue（二进制兼容 hftbase MultiWriterMultiReaderShmQueue）
// ============================================================

// C++ 源: hftbase/Ipc/include/multiwritermultireadershmqueue.h

// Header: 仅 head（8 bytes），初始值 1
// C++ 源: MultiWriterMultiReaderShmHeader
struct MWMRHeader {
    std::atomic<int64_t> head;
};

// QueueElem: data 在前，seqNo 在后
// C++ 源: QueueElem<T>
template<typename T>
struct QueueElem {
    T data;
    uint64_t seqNo;
};

// 向上取整到 2 的幂
// C++ 源: getMinHighestPowOf2()
inline int64_t next_pow2(int64_t v) {
    v--;
    v |= v >> 1; v |= v >> 2; v |= v >> 4;
    v |= v >> 8; v |= v >> 16; v |= v >> 32;
    return v + 1;
}

template<typename T>
class MWMRQueue {
public:
    // 创建队列（server 端）
    static MWMRQueue<T>* Create(int shmkey, int64_t requested_size) {
        int64_t size = next_pow2(requested_size);
        size_t total = sizeof(MWMRHeader) + size * sizeof(QueueElem<T>);

        void* addr = shm_create(shmkey, total);
        auto* q = new MWMRQueue<T>();
        q->init(addr, size);

        // 初始化 header
        q->header()->head.store(1, std::memory_order_relaxed);
        memset(q->m_updates, 0, size * sizeof(QueueElem<T>));

        return q;
    }

    // 打开已有队列（client 端）
    static MWMRQueue<T>* Open(int shmkey, int64_t requested_size) {
        int64_t size = next_pow2(requested_size);
        size_t total = sizeof(MWMRHeader) + size * sizeof(QueueElem<T>);

        void* addr = shm_open_existing(shmkey, total);
        auto* q = new MWMRQueue<T>();
        q->init(addr, size);

        // tail 追到当前 head（跳过历史数据）
        q->m_tail = q->header()->head.load(std::memory_order_relaxed);

        return q;
    }

    // Enqueue — 多生产者安全
    // C++ 源: multiwritermultireadershmqueue.h:118-133
    void enqueue(const T& value) {
        int64_t myHead = header()->head.fetch_add(1, std::memory_order_acq_rel);
        QueueElem<T>* slot = m_updates + (myHead & m_mask);
        memcpy(&(slot->data), &value, sizeof(T));
        asm volatile("" ::: "memory");  // compiler barrier
        slot->seqNo = myHead;
    }

    // IsEmpty — 检查是否有新数据
    // C++ 源: multiwritermultireadershmqueue.h:245-249
    bool isEmpty() const {
        return (m_updates + (m_tail & m_mask))->seqNo < (uint64_t)m_tail;
    }

    // Dequeue — 单消费者模式
    // C++ 源: multiwritermultireadershmqueue.h:204-211
    void dequeuePtr(T* data) {
        QueueElem<T>* slot = m_updates + (m_tail & m_mask);
        memcpy(data, &(slot->data), sizeof(T));
        m_tail = slot->seqNo + 1;
    }

    void close() {
        if (m_base) {
            shm_detach(m_base);
            m_base = nullptr;
        }
    }

private:
    void init(void* addr, int64_t size) {
        m_base = addr;
        m_updates = reinterpret_cast<QueueElem<T>*>(
            static_cast<char*>(addr) + sizeof(MWMRHeader));
        m_size = size;
        m_mask = size - 1;
        m_tail = 1;  // 默认初始值，Open() 会覆盖
    }

    MWMRHeader* header() {
        return reinterpret_cast<MWMRHeader*>(m_base);
    }

    void* m_base = nullptr;
    QueueElem<T>* m_updates = nullptr;
    int64_t m_size = 0;
    int64_t m_mask = 0;
    int64_t m_tail = 1;  // 进程本地，不在 SHM 中
};

// ============================================================
// 3. ClientStore（二进制兼容 hftbase LocklessShmClientStore）
// ============================================================

// C++ 源: hftbase/Ipc/include/locklessshmclientstore.h
struct ClientStoreData {
    std::atomic<int64_t> data;     // 当前计数器
    int64_t firstClientId;         // 初始值
};

class ClientStore {
public:
    static ClientStore* Create(int shmkey, int64_t initial_value = 0) {
        void* addr = shm_create(shmkey, sizeof(ClientStoreData));
        auto* cs = new ClientStore();
        cs->m_data = reinterpret_cast<ClientStoreData*>(addr);
        cs->m_data->data.store(initial_value, std::memory_order_relaxed);
        cs->m_data->firstClientId = initial_value;
        return cs;
    }

    static ClientStore* Open(int shmkey) {
        void* addr = shm_open_existing(shmkey, sizeof(ClientStoreData));
        auto* cs = new ClientStore();
        cs->m_data = reinterpret_cast<ClientStoreData*>(addr);
        return cs;
    }

    int64_t getClientIdAndIncrement() {
        return m_data->data.fetch_add(1, std::memory_order_acq_rel);
    }

    int64_t getClientId() const {
        return m_data->data.load(std::memory_order_acquire);
    }

private:
    ClientStoreData* m_data = nullptr;
};

} // namespace hftbase_compat
```

### 4.3 验证

此文件与 Go 的 `pkg/shm/mwmr_queue.go` 和 hftbase 原代码三方共用同一个 offset_check 验证流程：

```bash
# C++ offset_check（引用 hftbase 头文件）输出基准值
# Go offset_check 对比
# hftbase_shm.h 的 sizeof/offsetof 必须与基准值一致
```

---

## 5. counter_bridge.cpp 具体改动

### 5.1 头文件和类型定义（第 20-36 行区域）

```cpp
// ---- 删除 ----
#include "shm_queue.h"
#include "ors_gateway.h"
using OrderReqQueue = hft::shm::SPSCQueue<hft::ors::OrderRequestRaw, 4096>;
using OrderRespQueue = hft::shm::SPSCQueue<hft::ors::OrderResponseRaw, 4096>;

// ---- 替换为 ----
#include "hftbase_shm.h"
#include "hftbase_types.h"   // RequestMsg/ResponseMsg 定义（见 4.2 节）

using namespace hftbase_compat;
using ReqQueue  = MWMRQueue<illuminati::infra::RequestMsg>;
using RespQueue = MWMRQueue<illuminati::infra::ResponseMsg>;
```

### 5.2 hftbase 消息结构体

需要在 `gateway/include/hftbase_types.h` 中定义与 hftbase 二进制兼容的 `RequestMsg`/`ResponseMsg`。

可选两种方式：
- **方式 A**：直接 include hftbase 头文件（`#include "orderresponse.h"`），需要 `-I` 指向 hftbase
- **方式 B**：独立定义兼容结构体（无 hftbase 依赖）

推荐方式 B（与 Go 策略一致——独立复刻，offset_check 验证）：

```cpp
// gateway/include/hftbase_types.h
#pragma once
#include <cstdint>
#include <cstring>

namespace illuminati {
namespace infra {

// ============================================================
// 常量（与 hftbase/CommonUtils/include/constants.h 一致）
// ============================================================
static const int32_t ORDERID_RANGE = 1000000;
static const int MAX_ACCNTID_LEN = 10;
static const int MAX_SYMBOL_SIZE = 50;
static const int MAX_INSTRNAME_SIZE = 32;
static const int MAX_TRADE_ID_SIZE = 21;

// ============================================================
// RequestType 枚举
// ============================================================
enum RequestType {
    NEWORDER = 0,
    MODIFYORDER = 1,
    CANCELORDER = 2,
    ORDERSTATUS = 3,
    ORDERHISTORY = 4,
    STRATEGY = 5,
    // ... 更多见 orderresponse.h
};

// ============================================================
// ResponseType 枚举
// ============================================================
enum ResponseType {
    NEW_ORDER_CONFIRM = 0,
    MODIFY_ORDER_CONFIRM = 1,
    ORDER_REPLACE = 2,
    CANCEL_ORDER_CONFIRM = 3,
    TRADE_CONFIRM = 4,
    ORDER_ERROR = 5,
    CANCEL_ORDER_REJECT = 6,
    MODIFY_ORDER_REJECT = 7,
    ORS_REJECT = 8,
    RMS_REJECT = 9,
    // ... 更多见 orderresponse.h
};

// ============================================================
// PositionDirection 枚举
// ============================================================
enum PositionDirection {
    POS_OPEN = 10,
    POS_CLOSE = 11,
    POS_CLOSE_INTRADAY = 12,
};

// ============================================================
// OrderType 枚举
// ============================================================
enum OrderType {
    OT_LIMIT = 1,
    OT_MARKET = 2,
};

// ============================================================
// OrderDuration 枚举
// ============================================================
enum OrderDuration {
    OD_DAY = 0,
    OD_IOC = 1,
    OD_FOK = 2,
};

// ============================================================
// ContractDescription
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:107-115
// ============================================================
struct ContractDescription {
    char InstrumentName[MAX_INSTRNAME_SIZE];  // 32
    char Symbol[MAX_SYMBOL_SIZE];             // 50
    int32_t ExpiryDate;
    int32_t StrikePrice;
    char OptionType[2];
    int16_t CALevel;
};

// ============================================================
// RequestMsg
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:134-295
// ⚠️ __attribute__((aligned(64)))
// ============================================================
struct RequestMsg {
    ContractDescription Contract_Description;
    int32_t Request_Type;       // enum RequestType
    int32_t OrdType;            // enum OrderType
    int32_t Duration;           // enum OrderDuration
    int32_t PxType;             // enum PriceType
    int32_t PosDirection;       // enum PositionDirection
    uint32_t OrderID;           // clientId * 1000000 + seq
    int32_t Token;
    int32_t Quantity;
    int32_t QuantityFilled;
    int32_t DisclosedQnty;
    double Price;
    uint64_t TimeStamp;
    char AccountID[MAX_ACCNTID_LEN + 1];  // 11
    unsigned char Transaction_Type;        // 'B' 或 'S'
    unsigned char Exchange_Type;           // CHINA_SHFE=57 等
    char padding[20];
    char Product[32];
    int StrategyID;
} __attribute__((aligned(64)));

// ============================================================
// ResponseMsg
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:436-561
// ⚠️ 需要 offset_check 验证 padding
// ============================================================
struct ResponseMsg {
    int32_t Response_Type;          // enum ResponseType
    int32_t Child_Response;         // enum SubResponseType
    uint32_t OrderID;
    uint32_t ErrorCode;
    int32_t Quantity;
    double Price;
    uint64_t TimeStamp;
    unsigned char Side;             // 'B' 或 'S'
    char Symbol[MAX_SYMBOL_SIZE];   // 50
    char AccountID[MAX_ACCNTID_LEN + 1]; // 11
    double ExchangeOrderId;
    char ExchangeTradeId[MAX_TRADE_ID_SIZE]; // 21
    unsigned char OpenClose;        // OPEN=1, CLOSE=2, CLOSE_TODAY=3
    unsigned char ExchangeID;       // SHFE=1, INE=2, ...
    char Product[32];
    int StrategyID;
};

// ============================================================
// 交易所代码（行情用）
// ============================================================
static const unsigned char CHINA_SHFE  = 57;
static const unsigned char CHINA_CFFEX = 58;
static const unsigned char CHINA_ZCE   = 59;
static const unsigned char CHINA_DCE   = 60;
static const unsigned char CHINA_GFEX  = 61;

// ============================================================
// 交易所代码（回报用, TsExchangeID）
// ============================================================
static const unsigned char TSEXCH_SHFE  = 1;
static const unsigned char TSEXCH_INE   = 2;
static const unsigned char TSEXCH_CZCE  = 3;
static const unsigned char TSEXCH_DCE   = 4;
static const unsigned char TSEXCH_CFFEX = 5;
static const unsigned char TSEXCH_GFEX  = 6;

} // namespace infra
} // namespace illuminati
```

**⚠️ ResponseMsg 的 padding 需要 offset_check 精确验证后补齐。** 上述定义是框架，具体 padding 位置取决于编译器对齐规则。

### 5.3 新增全局变量和结构体

```cpp
// ---- 新增（在全局变量区域，约第 40-80 行）----

// 持仓结构体
// C++ 源: ors/Shengli/include/ORSServer.h:102-108
struct contractPos {
    int ONLongPos      = 0;   // 昨多仓
    int todayLongPos   = 0;   // 今多仓
    int ONShortPos     = 0;   // 昨空仓
    int todayShortPos  = 0;   // 今空仓
};

// 持仓 map
// C++ 源: ors/Shengli/include/ORSServer.h:425-427
static std::map<std::string, contractPos> g_mapContractPos;
static std::mutex g_posLock;

// 开平类型常量
// C++ 源: ors/China/src/ORSServer.cpp:28-30
static const int OPEN_ORDER        = 3;
static const int CLOSE_TODAY_FLAG  = 1;
static const int CLOSE_YESTD_FLAG = 2;

// 订单缓存（修改：增加 hftbase 字段）
struct CachedOrderInfo {
    uint32_t order_id;         // ★ 新增：hftbase uint32 OrderID
    int strategy_id;           // ★ 新增：int StrategyID
    std::string symbol;
    std::string exchange;
    unsigned char side;        // ★ 改为 char: 'B'/'S'
    std::string client_order_id;
    int openCloseFlag;         // ★ 新增：开平标志（OPEN_ORDER / CLOSE_TODAY_FLAG / CLOSE_YESTD_FLAG）
};

// SHM 配置
struct SHMConfig {
    int request_key    = 3872;
    int request_size   = 4096;
    int response_key   = 4872;
    int response_size  = 4096;
    int client_store_key = 5872;
};
```

### 5.4 SHM 初始化（替换第 589-604 行）

```cpp
// ---- 删除 ----
auto* req_queue = hft::shm::ShmManager::CreateOrOpenGeneric<
    hft::ors::OrderRequestRaw, 4096>("ors_request");
auto* resp_queue = hft::shm::ShmManager::CreateOrOpenGeneric<
    hft::ors::OrderResponseRaw, 4096>("ors_response");

// ---- 替换为 ----
SHMConfig shm_cfg;
// TODO: 从配置文件加载 shm_cfg

auto* req_queue = ReqQueue::Create(shm_cfg.request_key, shm_cfg.request_size);
std::cout << "[Main] ✅ Request MWMR queue ready (SysV key="
          << shm_cfg.request_key << ")" << std::endl;

auto* resp_queue = RespQueue::Create(shm_cfg.response_key, shm_cfg.response_size);
g_response_queue = resp_queue;
std::cout << "[Main] ✅ Response MWMR queue ready (SysV key="
          << shm_cfg.response_key << ")" << std::endl;

auto* client_store = ClientStore::Create(shm_cfg.client_store_key);
std::cout << "[Main] ✅ Client store ready (SysV key="
          << shm_cfg.client_store_key << ")" << std::endl;
```

### 5.5 SetCombOffsetFlag — 新增

从 `ors/China/src/ORSServer.cpp:488-605` 移植，适配 counter_bridge 的数据结构。

```cpp
// 新增函数
// C++ 源: ors/China/src/ORSServer.cpp:488-605
// C++ 源: ors/Shengli/src/ORSServer.cpp:672-779
void SetCombOffsetFlag(
    const illuminati::infra::RequestMsg* request,
    int& openCloseFlag,
    unsigned char exchangeType)
{
    std::string symbol(request->Contract_Description.Symbol);
    bool isSHFE = (exchangeType == illuminati::infra::CHINA_SHFE);
    // INE（上海能源中心）也区分平今/平昨，与 SHFE 规则相同
    // bool isINE = ...;

    std::lock_guard<std::mutex> lock(g_posLock);
    auto& pos = g_mapContractPos[symbol];

    if (request->Transaction_Type == 'B') {
        // 买入 → 先尝试平空仓

        // 1. 先平今仓（SHFE/INE 区分平今）
        if (request->Quantity <= pos.todayShortPos) {
            openCloseFlag = isSHFE ? CLOSE_TODAY_FLAG : CLOSE_YESTD_FLAG;
            pos.todayShortPos -= request->Quantity;
            return;
        }

        // 2. 再平昨仓
        if (request->Quantity <= pos.ONShortPos) {
            openCloseFlag = CLOSE_YESTD_FLAG;
            pos.ONShortPos -= request->Quantity;
            return;
        }

        // 3. 开新仓
        openCloseFlag = OPEN_ORDER;

    } else {
        // 卖出 → 先尝试平多仓

        // 1. 先平今仓
        if (request->Quantity <= pos.todayLongPos) {
            openCloseFlag = isSHFE ? CLOSE_TODAY_FLAG : CLOSE_YESTD_FLAG;
            pos.todayLongPos -= request->Quantity;
            return;
        }

        // 2. 再平昨仓
        if (request->Quantity <= pos.ONLongPos) {
            openCloseFlag = CLOSE_YESTD_FLAG;
            pos.ONLongPos -= request->Quantity;
            return;
        }

        // 3. 开新仓
        openCloseFlag = OPEN_ORDER;
    }
}
```

### 5.6 updatePosition — 新增

从 `ors/China/src/ORSServer.cpp:1186-1281` 移植。

```cpp
// 新增函数
// C++ 源: ors/China/src/ORSServer.cpp:1186-1281
// C++ 源: ors/Shengli/src/ORSServer.cpp:1637-1736
void updatePosition(
    const illuminati::infra::ResponseMsg* resp,
    const CachedOrderInfo& info)
{
    std::lock_guard<std::mutex> lock(g_posLock);
    auto& pos = g_mapContractPos[info.symbol];

    if (resp->Response_Type == illuminati::infra::TRADE_CONFIRM) {
        // 成交：开仓时增加持仓
        if (info.openCloseFlag == OPEN_ORDER) {
            if (resp->Side == 'B') {
                pos.todayLongPos += resp->Quantity;
            } else {
                pos.todayShortPos += resp->Quantity;
            }
        }
        // 平仓：持仓已在 SetCombOffsetFlag 中扣减，不操作

    } else if (resp->Response_Type == illuminati::infra::ORDER_ERROR ||
               resp->Response_Type == illuminati::infra::CANCEL_ORDER_CONFIRM) {
        // 拒单/撤单：解冻持仓（反向加回）
        int qty = resp->Quantity;  // 未成交数量

        if (info.openCloseFlag == CLOSE_TODAY_FLAG) {
            if (info.side == 'B') {
                pos.todayShortPos += qty;
            } else {
                pos.todayLongPos += qty;
            }
        } else if (info.openCloseFlag == CLOSE_YESTD_FLAG) {
            if (info.side == 'B') {
                pos.ONShortPos += qty;
            } else {
                pos.ONLongPos += qty;
            }
        }
        // OPEN_ORDER 拒单/撤单：不操作（没有冻结过持仓）
    }
}
```

### 5.7 OrderRequestProcessor 改造（替换第 446-567 行）

```cpp
// ---- 完整替换 OrderRequestProcessor ----
void OrderRequestProcessor(ReqQueue* req_queue) {
    std::cout << "[Processor] Order request processor started (MWMR mode)" << std::endl;

    illuminati::infra::RequestMsg req;

    while (g_running.load()) {
        if (!req_queue->isEmpty()) {
            req_queue->dequeuePtr(&req);
            g_stats.total_orders++;

            // 提取符号
            std::string symbol(req.Contract_Description.Symbol);

            // 获取对应的券商插件
            ITDPlugin* broker = GetBrokerForSymbol(symbol);
            if (!broker) {
                std::cerr << "[Processor] ❌ No broker for: " << symbol << std::endl;
                g_stats.failed_orders++;

                // 发送 ORS_REJECT 回报
                illuminati::infra::ResponseMsg resp;
                std::memset(&resp, 0, sizeof(resp));
                resp.Response_Type = illuminati::infra::ORS_REJECT;
                resp.OrderID = req.OrderID;
                resp.ErrorCode = 1;
                resp.StrategyID = req.StrategyID;
                std::strncpy(resp.Symbol, symbol.c_str(), sizeof(resp.Symbol) - 1);
                g_response_queue->enqueue(resp);
                continue;
            }

            // ★ 开平自动推断（模式2）
            int openCloseFlag = OPEN_ORDER;
            SetCombOffsetFlag(&req, openCloseFlag, req.Exchange_Type);

            // 转换为 ITDPlugin 统一格式
            hft::plugin::OrderRequest unified_req;
            std::memset(&unified_req, 0, sizeof(unified_req));

            std::strncpy(unified_req.symbol, symbol.c_str(), sizeof(unified_req.symbol) - 1);

            // 交易所代码转换: Exchange_Type (byte) → 字符串
            switch (req.Exchange_Type) {
                case illuminati::infra::CHINA_SHFE:  std::strcpy(unified_req.exchange, "SHFE"); break;
                case illuminati::infra::CHINA_CFFEX: std::strcpy(unified_req.exchange, "CFFEX"); break;
                case illuminati::infra::CHINA_ZCE:   std::strcpy(unified_req.exchange, "CZCE"); break;
                case illuminati::infra::CHINA_DCE:   std::strcpy(unified_req.exchange, "DCE"); break;
                case illuminati::infra::CHINA_GFEX:  std::strcpy(unified_req.exchange, "GFEX"); break;
                default: std::strcpy(unified_req.exchange, "SHFE"); break;
            }

            // 方向转换: 'B'/'S' → BUY/SELL
            unified_req.direction = (req.Transaction_Type == 'B')
                ? hft::plugin::OrderDirection::BUY
                : hft::plugin::OrderDirection::SELL;

            // 开平转换: SetCombOffsetFlag 结果 → OffsetFlag
            switch (openCloseFlag) {
                case OPEN_ORDER:       unified_req.offset = hft::plugin::OffsetFlag::OPEN; break;
                case CLOSE_TODAY_FLAG: unified_req.offset = hft::plugin::OffsetFlag::CLOSE_TODAY; break;
                case CLOSE_YESTD_FLAG: unified_req.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY; break;
                default:               unified_req.offset = hft::plugin::OffsetFlag::OPEN; break;
            }

            // 价格
            unified_req.price_type = (req.OrdType == illuminati::infra::OT_MARKET)
                ? hft::plugin::PriceType::MARKET
                : hft::plugin::PriceType::LIMIT;
            unified_req.price = req.Price;
            unified_req.volume = static_cast<uint32_t>(req.Quantity);

            // OrderID → 字符串 client_order_id（ITDPlugin 用字符串）
            snprintf(unified_req.client_order_id,
                     sizeof(unified_req.client_order_id),
                     "%u", req.OrderID);

            std::cout << "[Processor] 📤 " << broker->GetPluginName() << ": "
                      << symbol << " "
                      << (req.Transaction_Type == 'B' ? "BUY" : "SELL")
                      << " " << req.Quantity << "@" << req.Price
                      << " (OID=" << req.OrderID << " flag=" << openCloseFlag << ")"
                      << std::endl;

            // 发到券商
            try {
                std::string broker_order_id = broker->SendOrder(unified_req);

                if (!broker_order_id.empty()) {
                    g_stats.success_orders++;

                    // 缓存订单信息
                    std::lock_guard<std::mutex> lock(g_orders_mutex);
                    CachedOrderInfo info;
                    info.order_id = req.OrderID;
                    info.strategy_id = req.StrategyID;
                    info.symbol = symbol;
                    info.exchange = unified_req.exchange;
                    info.side = req.Transaction_Type;
                    info.client_order_id = unified_req.client_order_id;
                    info.openCloseFlag = openCloseFlag;
                    g_order_map[broker_order_id] = info;
                } else {
                    g_stats.failed_orders++;
                    // 发送拒绝回报 + 解冻持仓
                    illuminati::infra::ResponseMsg resp;
                    std::memset(&resp, 0, sizeof(resp));
                    resp.Response_Type = illuminati::infra::ORDER_ERROR;
                    resp.OrderID = req.OrderID;
                    resp.ErrorCode = 1;
                    resp.Quantity = req.Quantity;
                    resp.Side = req.Transaction_Type;
                    resp.StrategyID = req.StrategyID;
                    std::strncpy(resp.Symbol, symbol.c_str(), sizeof(resp.Symbol) - 1);

                    CachedOrderInfo tmpInfo;
                    tmpInfo.symbol = symbol;
                    tmpInfo.side = req.Transaction_Type;
                    tmpInfo.openCloseFlag = openCloseFlag;
                    updatePosition(&resp, tmpInfo);

                    g_response_queue->enqueue(resp);
                }
            } catch (const std::exception& e) {
                g_stats.failed_orders++;
                std::cerr << "[Processor] ❌ Exception: " << e.what() << std::endl;
            }
        } else {
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }
}
```

### 5.8 OnBrokerOrderCallback 改造（替换第 78-180 行）

```cpp
// ---- 完整替换 OnBrokerOrderCallback ----
void OnBrokerOrderCallback(const hft::plugin::OrderInfo& order_info) {
    illuminati::infra::ResponseMsg resp;
    std::memset(&resp, 0, sizeof(resp));

    // 从缓存找回订单信息
    CachedOrderInfo cached_info;
    {
        std::lock_guard<std::mutex> lock(g_orders_mutex);
        auto it = g_order_map.find(order_info.order_id);
        if (it != g_order_map.end()) {
            cached_info = it->second;
        } else {
            std::cerr << "[Bridge] ⚠ Order not in cache: " << order_info.order_id << std::endl;
            return;
        }
    }

    // 填充 ResponseMsg
    resp.OrderID = cached_info.order_id;          // uint32
    resp.StrategyID = cached_info.strategy_id;    // int
    resp.Side = cached_info.side;                 // 'B' 或 'S'
    std::strncpy(resp.Symbol, cached_info.symbol.c_str(), sizeof(resp.Symbol) - 1);

    // 状态映射: plugin::OrderStatus → hftbase ResponseType
    switch (order_info.status) {
        case hft::plugin::OrderStatus::ACCEPTED:
        case hft::plugin::OrderStatus::SUBMITTED:
            resp.Response_Type = illuminati::infra::NEW_ORDER_CONFIRM;
            break;

        case hft::plugin::OrderStatus::PARTIAL_FILLED:
        case hft::plugin::OrderStatus::FILLED:
            resp.Response_Type = illuminati::infra::TRADE_CONFIRM;
            resp.Quantity = order_info.traded_volume;
            resp.Price = order_info.price;
            break;

        case hft::plugin::OrderStatus::CANCELED:
            resp.Response_Type = illuminati::infra::CANCEL_ORDER_CONFIRM;
            resp.Quantity = order_info.volume - order_info.traded_volume; // 未成交量
            break;

        case hft::plugin::OrderStatus::REJECTED:
        case hft::plugin::OrderStatus::ERROR:
            resp.Response_Type = illuminati::infra::ORDER_ERROR;
            resp.ErrorCode = 1;
            resp.Quantity = order_info.volume;
            break;

        default:
            resp.Response_Type = illuminati::infra::ORDER_ERROR;
            break;
    }

    resp.TimeStamp = order_info.update_time;

    // ★ 更新持仓
    updatePosition(&resp, cached_info);

    // 写入 MWMR 回报队列
    g_response_queue->enqueue(resp);

    std::cout << "[Bridge] Response: OID=" << resp.OrderID
              << " type=" << resp.Response_Type
              << " qty=" << resp.Quantity << std::endl;
}
```

### 5.9 删除 HTTP 持仓端点

```cpp
// ---- 删除以下内容 ----

// 1. 删除 HandlePositionQuery 函数（counter_bridge.cpp:299-401）
//    该函数通过 HTTP 向 Go 返回 CTP 持仓，改造后不再需要

// 2. 删除路由注册（counter_bridge.cpp:408）
//    g_http_server->Get("/positions", HandlePositionQuery);

// 3. 如果 HTTP server 没有其他端点使用，删除 HTTP server 初始化代码
//    包括 httplib.h 引用、g_http_server 创建、listen 线程等
```

### 5.10 持仓文件加载（新增，可选）

```cpp
// 新增函数：从 CSV 加载初始持仓
// 格式: symbol,ONLong,todayLong,ONShort,todayShort
// 例如: ag2506,0,3,0,5
void loadPositionFile(const std::string& filename) {
    if (filename.empty()) return;
    std::ifstream file(filename);
    if (!file.is_open()) {
        std::cerr << "[Position] ⚠ Cannot open: " << filename << std::endl;
        return;
    }
    std::string line;
    while (std::getline(file, line)) {
        // 解析 CSV 行，填入 g_mapContractPos
        // ...
    }
    std::cout << "[Position] ✅ Loaded " << g_mapContractPos.size()
              << " positions from " << filename << std::endl;
}
```

---

## 6. CMakeLists.txt 改动

```cmake
# 新增 include 路径
target_include_directories(counter_bridge PRIVATE
    ${CMAKE_SOURCE_DIR}/include           # 原有
    ${CMAKE_SOURCE_DIR}/include/plugin    # 原有
    # hftbase_shm.h 和 hftbase_types.h 放在 include/ 下，无需额外路径
)

# 无需新增链接库（SysV SHM 用 syscall，不需要额外 .so）
```

---

## 7. 配置文件

### 7.1 counter_bridge 配置（新增 SHM 段）

counter_bridge 当前从命令行参数获取 broker 配置。SHM key 可以通过配置文件或命令行参数传入：

```yaml
# config/counter_bridge.yaml（新增）
shm:
  request_key: 3872
  request_size: 4096
  response_key: 4872
  response_size: 4096
  client_store_key: 5872

position:
  file: ""                    # 初始持仓文件路径（可选）
```

或保持命令行风格：

```bash
# 启动方式不变，增加 --shm-config 参数
./counter_bridge ctp:/path/to/ctp.yaml --shm-config /path/to/shm.yaml
```

### 7.2 Go trader 配置

Go trader 的 YAML 中 SHM key 必须与 counter_bridge 一致（见 `tbsrc-golang_v2_架构更新` 文档）。

---

## 8. 实施顺序

```
前置: Go MWMR 实现完成（Phase 1.2-1.5）

步骤 1: 编写 hftbase_shm.h + hftbase_types.h
    └─ offset_check 三方验证（hftbase vs hftbase_shm.h vs Go）

步骤 2: 删除 HTTP 持仓端点
    ├─ 删除 HandlePositionQuery 函数
    ├─ 删除 /positions 路由注册
    └─ 清理 HTTP server（如无其他端点使用）

步骤 3: 改造 counter_bridge SHM 初始化
    └─ POSIX SPSC → SysV MWMR，确认能创建/打开队列

步骤 4: 改造消息转换层
    ├─ OrderRequestProcessor: RequestMsg → ITDPlugin OrderRequest
    └─ OnBrokerOrderCallback: OrderInfo → ResponseMsg

步骤 5: 新增 SetCombOffsetFlag + mapContractPos + updatePosition
    └─ 从 ors/China 移植

步骤 6: 端到端测试
    └─ go_trader ←→ [SysV MWMR] ←→ counter_bridge ←→ Simulator plugin

步骤 7（后续）: 可选增强
    ├─ RMS 基础风控
    ├─ OrderCrossCheck
    └─ 持仓持久化

步骤 8（后续）: Go 端清理
    ├─ 删除 ORSClient.QueryPositions()
    ├─ 删除 positionsByExchange 相关逻辑
    └─ /api/v1/positions 改为从策略内部状态读取
```

---

## 9. 验证标准

- ✅ HTTP `/positions` 端点已移除，counter_bridge 不再提供 HTTP 服务
- ✅ Go 写 `RequestMsg` → counter_bridge 正确读出并转发到 ITDPlugin
- ✅ ITDPlugin 回报 → counter_bridge 正确写 `ResponseMsg` → Go 正确读出
- ✅ Go 通过 MWMR response queue 的 TRADE_CONFIRM 正确累计持仓（与原 C++ 策略行为一致）
- ✅ `SetCombOffsetFlag` 自动推断开平方向正确（SHFE 平今/平昨/开仓）
- ✅ `updatePosition` 在成交/拒单/撤单时正确更新持仓
- ✅ 多个 Go trader 进程可同时连接（MWMR 多写者安全）
- ✅ OrderID 整数格式全链路正确传递和过滤

---

## 参考资料

### 原 C++ 持仓跟踪（独立模型，无 HTTP 查询）

- 策略端持仓跟踪: `tbsrc/Strategies/ExecutionStrategy.cpp` ProcessTrade() — TRADE_CONFIRM 累计
- 策略持仓字段: `tbsrc/Strategies/include/ExecutionStrategy.h:111-114` — m_netpos, m_netpos_pass, m_netpos_agg
- 策略持仓加载: `tbsrc/Strategies/PairwiseArbStrategy.cpp:30-62` — 读 daily_init 文件
- 策略持仓保存: `tbsrc/Strategies/PairwiseArbStrategy.cpp:653-686` — SaveMatrix2()
- 跨策略持仓共享: `tbsrc/Strategies/PairwiseArbStrategy.cpp:885-900` — tcache SHM
- 持仓监控发布: `tbsrc/Strategies/ExecutionStrategy.cpp:133` — memlog SHM

### 原 ORS 持仓跟踪

- 原 SetCombOffsetFlag: `ors/China/src/ORSServer.cpp:488-605`
- 原 updatePosition: `ors/China/src/ORSServer.cpp:1186-1281`
- 原 mapContractPos: `ors/Shengli/include/ORSServer.h:422-431`

### 当前系统 HTTP 持仓端点（待删除）

- counter_bridge HTTP 端点: `gateway/src/counter_bridge.cpp:299-408` — HandlePositionQuery + 路由注册
- Go 调用端: `golang/pkg/client/ors_client.go:311-383` — QueryPositions()
- Go 存储: `golang/pkg/trader/trader.go:38-40` — positionsByExchange
- Go REST API: `golang/pkg/trader/api.go:101-103, 400-509` — /api/v1/positions

### 基础设施

- 原 MWMR Queue: `hftbase/Ipc/include/multiwritermultireadershmqueue.h`
- MWMR Go 复刻: `docs/系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md`
- 架构更新: `docs/系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md`
- 当前 counter_bridge: `gateway/src/counter_bridge.cpp`
- ITDPlugin 接口: `gateway/include/plugin/td_plugin_interface.h`

---

**最后更新**: 2026-02-13（v1.1: 新增第 2 节 HTTP 持仓端点移除分析）
