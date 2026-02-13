# hftbase MWMR Queue Go 复刻技术规格

**文档日期**: 2026-02-13
**版本**: v1.0
**相关模块**: tbsrc-golang/pkg/shm

---

## 概述

本文档详细描述如何在纯 Go 中复刻 hftbase 的 `MultiWriterMultiReaderShmQueue`，实现与 C++ 原 ORS 进程的**二进制兼容**。Go trader 通过读写同一块 SysV 共享内存与 C++ ORS 交互，无需 CGO。

**设计目标**：
- Go 写入的 `RequestMsg` 能被 C++ ORS 正确读取
- C++ ORS 写入的 `ResponseMsg`/`MarketUpdateNew` 能被 Go 正确读取
- 多个 Go trader 进程可与 C++ trader 进程共享同一组 MWMR 队列

---

## 1. C++ 原代码位置

| 组件 | 文件路径 | 行数 |
|------|---------|------|
| MWMR Queue | `hftbase/Ipc/include/multiwritermultireadershmqueue.h` | 261 |
| SHM Allocator | `hftbase/Ipc/include/shmallocator.h` | 92 |
| SharedMemory | `hftbase/Ipc/include/sharedmemory.h` | 120 |
| ClientStore | `hftbase/Ipc/include/locklessshmclientstore.h` | 91 |
| ShmManager | `hftbase/Ipc/include/shmmanager.h` | ~950 |
| RequestMsg | `hftbase/CommonUtils/include/orderresponse.h` | 295 |
| ResponseMsg | `hftbase/CommonUtils/include/orderresponse.h` | 436-561 |
| MarketUpdateNew | `hftbase/CommonUtils/include/marketupdateNew.h` | 477-557 |
| Constants | `hftbase/CommonUtils/include/constants.h` | 16 |

---

## 2. SysV 共享内存（非 POSIX SHM）

### 2.1 C++ 原实现

hftbase 使用 **SysV IPC** 共享内存（`shmget`/`shmat`），与新 gateway 的 POSIX SHM（`shm_open`/`mmap`）完全不同。

```cpp
// hftbase/Ipc/include/sharedmemory.h
// 创建或打开共享内存段
m_shmid = shmget(shmkey, m_shmsize, flag);  // flag = IPC_CREAT | 0666
m_shmadr = shmat(m_shmid, NULL, 0);         // 映射到进程地址空间

// 大小按页面对齐
size_t m_shmsize = size_in + page_size - (size_in % page_size);
```

### 2.2 Go 实现

Go 标准库 `syscall` 包支持 SysV SHM：

```go
// pkg/shm/sysv.go

import "syscall"

// 创建共享内存（ORS server 端调用）
func shmCreate(key int, size int) (uintptr, error) {
    shmid, _, errno := syscall.Syscall(syscall.SYS_SHMGET,
        uintptr(key), uintptr(size), uintptr(syscall.IPC_CREAT|0666))
    if errno != 0 {
        return 0, errno
    }
    addr, _, errno := syscall.Syscall(syscall.SYS_SHMAT,
        shmid, 0, 0)
    if errno != 0 {
        return 0, errno
    }
    return addr, nil
}

// 打开已存在的共享内存（Go trader 客户端调用）
func shmOpen(key int, size int) (uintptr, error) {
    shmid, _, errno := syscall.Syscall(syscall.SYS_SHMGET,
        uintptr(key), uintptr(size), uintptr(0666))
    if errno != 0 {
        return 0, errno
    }
    addr, _, errno := syscall.Syscall(syscall.SYS_SHMAT,
        shmid, 0, 0)
    if errno != 0 {
        return 0, errno
    }
    return addr, nil
}

// 断开映射
func shmDetach(addr uintptr) error {
    _, _, errno := syscall.Syscall(syscall.SYS_SHMDT, addr, 0, 0)
    if errno != 0 {
        return errno
    }
    return nil
}
```

### 2.3 平台差异

| 平台 | SysV SHM 支持 | 说明 |
|------|-------------|------|
| Linux/CentOS | 完全支持 | 生产环境，`syscall.SYS_SHMGET` 等可用 |
| macOS (darwin) | 部分支持 | 开发环境，SysV SHM 可用但 segment 大小有限制 |

两平台的 syscall 编号不同，但 Go 的 `syscall` 包已处理。需要平台分离文件：
- `sysv_linux.go` — Linux syscall 常量
- `sysv_darwin.go` — macOS syscall 常量

---

## 3. MWMR Queue 内存布局

### 3.1 SHM 段整体结构

```
SysV SHM 段 (key = 配置中的 REQUEST_SHMKEY / RESPONSE_SHMKEY / MD_SHMKEY):

Offset 0:
┌──────────────────────────────────────────────────┐
│ Header (8 bytes):                                 │
│   atomic<int64_t> head    // 写入游标（初始值=1） │
├──────────────────────────────────────────────────┤
│ Slot[0]:  QueueElem<T>                            │
│   ├─ T data              // sizeof(T) bytes       │
│   └─ uint64_t seqNo      // 8 bytes               │
├──────────────────────────────────────────────────┤
│ Slot[1]:  QueueElem<T>                            │
│   ├─ T data                                       │
│   └─ uint64_t seqNo                               │
├──────────────────────────────────────────────────┤
│ ...                                               │
├──────────────────────────────────────────────────┤
│ Slot[size-1]: QueueElem<T>                        │
│   ├─ T data                                       │
│   └─ uint64_t seqNo                               │
└──────────────────────────────────────────────────┘
```

**关键细节**：

| 属性 | 值 | 来源 |
|------|---|------|
| Header 大小 | 8 bytes（仅一个 `atomic<int64_t> head`） | `MultiWriterMultiReaderShmHeader` |
| Slot 布局 | **data 在前，seqNo 在后** | `QueueElem<T> { T data; uint64_t seqNo; }` |
| size | 配置值向上取整到 2 的幂 | `getMinHighestPowOf2()` |
| mask | size - 1 | 用于位运算取模 |
| head 初始值 | **1**（不是 0） | `MultiWriterMultiReaderShmHeader()` 构造函数 |
| tail 初始值 | **1**（或等于 head 值，取决于 `_TAIL_Q_START` 宏） | `multiwritermultireadershmqueue.h:97-100` |
| tail 存储位置 | **进程本地内存**（不在 SHM 中） | `std::atomic<int64_t> tail` 是类成员变量 |
| SHM 总大小 | `sizeof(Header) + size * sizeof(QueueElem<T>)`，按页面对齐 | `shmallocator.h:52`、`sharedmemory.h:52` |

### 3.2 QueueElem 布局详解

```
// C++:
template <typename T>
struct QueueElem {
    T data;          // offset 0, sizeof(T) bytes
    uint64_t seqNo;  // offset sizeof(T), 8 bytes
};
// 注意：没有 padding（T 本身已经是对齐的）
```

**⚠️ 上次计划中 seqNo 和 data 的顺序写反了。正确顺序是 data 在前、seqNo 在后。**

### 3.3 Go 结构体定义

```go
// pkg/shm/mwmr_queue.go

// MWMRHeader 对应 C++: MultiWriterMultiReaderShmHeader
// 只有一个 atomic head，8 字节
// C++ 源: hftbase/Ipc/include/multiwritermultireadershmqueue.h:15-23
type MWMRHeader struct {
    Head int64 // atomic<int64_t>，初始值 1
}

// QueueElem 对应 C++: QueueElem<T>
// C++ 源: hftbase/Ipc/include/multiwritermultireadershmqueue.h:26-30
// ⚠️ data 在前，seqNo 在后（不是反过来）
// Go 中不直接定义泛型 QueueElem，而是通过 offset 计算访问
```

---

## 4. MWMR Queue 操作实现

### 4.1 核心操作（C++ → Go 对照）

#### Enqueue（写入，多生产者安全）

```cpp
// C++ 原代码: multiwritermultireadershmqueue.h:118-133
inline void enqueue(const T &value) {
    int64_t myHead = header->head.fetch_add(1, memory_order_acq_rel);
    QueueElem<T> *slot = m_updates + (myHead & (m_size - 1));
    memcpy(&(slot->data), &value, sizeof(T));
    asm volatile("" ::: "memory");  // compiler barrier
    slot->seqNo = myHead;
}
```

```go
// Go 翻译
func (q *MWMRQueue[T]) Enqueue(value *T) {
    // C++: myHead = header->head.fetch_add(1, memory_order_acq_rel)
    myHead := atomic.AddInt64(q.headPtr(), 1) - 1  // AddInt64 返回新值，需要 -1 得到 fetch_add 语义

    // C++: slot = m_updates + (myHead & (m_size - 1))
    slot := q.slotDataPtr(myHead & q.mask)

    // C++: memcpy(&(slot->data), &value, sizeof(T))
    copyT(slot, value) // unsafe memmove

    // C++: asm volatile("" ::: "memory") — compiler barrier
    // Go 中 atomic.StoreInt64 自带 barrier

    // C++: slot->seqNo = myHead
    seqNoPtr := q.slotSeqNoPtr(myHead & q.mask)
    atomic.StoreInt64(seqNoPtr, myHead)  // release 语义
}
```

#### IsEmpty（检查是否有新数据）

```cpp
// C++ 原代码: multiwritermultireadershmqueue.h:245-249
inline bool isEmpty() {
    return (m_updates + (tail & (m_size - 1)))->seqNo < tail;
}
```

```go
// Go 翻译
func (q *MWMRQueue[T]) IsEmpty() bool {
    // C++: (m_updates + (tail & mask))->seqNo < tail
    seqNoPtr := q.slotSeqNoPtr(q.localTail & q.mask)
    seqNo := atomic.LoadInt64(seqNoPtr)  // acquire 语义
    return seqNo < q.localTail
}
```

#### DequeuePtr（读取，单消费者模式）

```cpp
// C++ 原代码: multiwritermultireadershmqueue.h:204-211
inline void dequeuePtr(T *data) {
    QueueElem<T> *value = m_updates + (tail.load(memory_order_relaxed) & (m_size - 1));
    memcpy(data, &(value->data), sizeof(T));
    tail.store(value->seqNo + 1, memory_order_relaxed);
}
```

```go
// Go 翻译
func (q *MWMRQueue[T]) Dequeue(data *T) {
    // C++: value = m_updates + (tail & (m_size - 1))
    slot := q.slotDataPtr(q.localTail & q.mask)

    // C++: memcpy(data, &(value->data), sizeof(T))
    copyT(data, slot)

    // C++: tail = value->seqNo + 1
    seqNoPtr := q.slotSeqNoPtr(q.localTail & q.mask)
    q.localTail = atomic.LoadInt64(seqNoPtr) + 1
}
```

#### DequeuePtrBlock（读取，多消费者模式 — 阻塞等待）

```cpp
// C++ 原代码: multiwritermultireadershmqueue.h:220-243
inline void dequeuePtrBlock(T *data, bool &isLive, callback) {
    int myTail = tail.fetch_add(1, memory_order_relaxed);
    QueueElem<T> *slot = m_updates + (myTail & (m_size - 1));
    while (slot->seqNo < myTail && isLive) {
        asm volatile("pause" ::: "memory");  // x86 PAUSE
    }
    memcpy(data, &(slot->data), sizeof(T));
}
```

**注意**：当前架构中，Go trader 作为单消费者使用 `Dequeue`（非阻塞），不需要 `DequeuePtrBlock`。原 ORS 使用 `startMonitorAsyncORSRequest` 中的 `dequeuePtr` 读请求，也是单消费者模式。

### 4.2 Go 泛型实现

```go
// pkg/shm/mwmr_queue.go

type MWMRQueue[T any] struct {
    base     uintptr  // SysV SHM shmat 返回地址
    elemSize uintptr  // sizeof(QueueElem<T>) = unsafe.Sizeof(T{}) + 8
    size     int64    // 队列容量（2的幂）
    mask     int64    // size - 1

    // 单读者优化：本地 tail（不在 SHM 中）
    localTail int64
}

// headPtr 返回 SHM 中 Header.head 的指针
func (q *MWMRQueue[T]) headPtr() *int64 {
    return (*int64)(unsafe.Pointer(q.base))  // offset 0
}

// slotDataPtr 返回 slot[index].data 的指针
func (q *MWMRQueue[T]) slotDataPtr(index int64) *T {
    // SHM 布局: [Header(8 bytes)][Slot[0]][Slot[1]]...
    // Slot offset = 8 + index * elemSize
    offset := uintptr(8) + uintptr(index)*q.elemSize
    return (*T)(unsafe.Pointer(q.base + offset))
}

// slotSeqNoPtr 返回 slot[index].seqNo 的指针
func (q *MWMRQueue[T]) slotSeqNoPtr(index int64) *int64 {
    // seqNo 在 data 之后：offset = 8 + index * elemSize + sizeof(T)
    offset := uintptr(8) + uintptr(index)*q.elemSize + q.elemSize - 8
    return (*int64)(unsafe.Pointer(q.base + offset))
}
```

### 4.3 内存拷贝辅助函数

```go
// copyT 执行 unsafe 内存拷贝（等价于 C++ memcpy）
func copyT[T any](dst, src *T) {
    size := unsafe.Sizeof(*dst)
    dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(dst)), size)
    srcSlice := unsafe.Slice((*byte)(unsafe.Pointer(src)), size)
    copy(dstSlice, srcSlice)
}
```

---

## 5. ClientStore（客户端ID分配）

### 5.1 C++ 内存布局

```cpp
// hftbase/Ipc/include/locklessshmclientstore.h:21-25
struct ClientData {
    std::atomic<IntType> data          __attribute__((aligned(sizeof(IntType))));  // 8 bytes
    IntType              firstCliendId __attribute__((aligned(sizeof(IntType))));  // 8 bytes
};
// IntType = uint64_t（见 shmmanager.h 中的实例化）
// 总大小: 16 bytes
```

### 5.2 Go 实现

```go
// pkg/shm/client_store.go

// ClientStore 对应 C++: LocklessShmClientStore<uint64_t>
// C++ 源: hftbase/Ipc/include/locklessshmclientstore.h
type ClientStore struct {
    base uintptr // shmat 返回地址
}

// ClientStoreData 对应 C++: ClientData { atomic<uint64_t> data; uint64_t firstCliendId; }
// SHM 内存布局（16 bytes）：
//   offset 0: atomic<uint64_t> data          — 当前客户端计数器
//   offset 8: uint64_t         firstCliendId — 初始客户端ID

// C++: getClientIdAndIncrement() → data.fetch_add(1, memory_order_acq_rel)
func (cs *ClientStore) GetClientIDAndIncrement() int64 {
    ptr := (*int64)(unsafe.Pointer(cs.base))
    return atomic.AddInt64(ptr, 1) - 1  // fetch_add 语义
}

// C++: getClientId() → data.load(memory_order_acquire)
func (cs *ClientStore) GetClientID() int64 {
    ptr := (*int64)(unsafe.Pointer(cs.base))
    return atomic.LoadInt64(ptr)
}
```

---

## 6. 消息结构体（与 C++ 二进制兼容）

### 6.1 RequestMsg

```cpp
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:134-295
// ⚠️ __attribute__((aligned(64))) — 整个结构体 64 字节对齐

struct ContractDescription {
    char InstrumentName[32];     // MAX_INSTRNAME_SIZE = 32
    char Symbol[50];             // MAX_SYMBOL_SIZE = 50
    int32_t ExpiryDate;          // 4 bytes
    int32_t StrikePrice;         // 4 bytes
    char OptionType[2];          // 2 bytes
    int16_t CALevel;             // 2 bytes
};  // 总计: 32+50+4+4+2+2 = 94 bytes，可能有 padding

struct RequestMsg {
    ContractDescription Contract_Description;  // ~94 bytes + padding
    RequestType Request_Type;      // enum (4 bytes)
    OrderType OrdType;             // enum (4 bytes)
    OrderDuration Duration;        // enum (4 bytes)
    PriceType PxType;              // enum (4 bytes)
    PositionDirection PosDirection; // enum (4 bytes)
    uint32_t OrderID;              // 4 bytes
    int32_t Token;                 // 4 bytes
    int32_t Quantity;              // 4 bytes
    int32_t QuantityFilled;        // 4 bytes
    int32_t DisclosedQnty;         // 4 bytes
    double Price;                  // 8 bytes
    uint64_t TimeStamp;            // 8 bytes
    char AccountID[11];            // 11 bytes
    unsigned char Transaction_Type;// 1 byte  ('B' 或 'S')
    unsigned char Exchange_Type;   // 1 byte  (CHINA_SHFE=57 等)
    char padding[20];              // 20 bytes
    char Product[32];              // 32 bytes
    int StrategyID;                // 4 bytes
} __attribute__((aligned(64)));
```

```go
// pkg/shm/types.go

// ContractDescription 对应 C++: ContractDescription
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:107-115
type ContractDescription struct {
    InstrumentName [32]byte   // char InstrumentName[32]
    Symbol         [50]byte   // char Symbol[50]   (MAX_SYMBOL_SIZE=50)
    ExpiryDate     int32      // int32_t
    StrikePrice    int32      // int32_t
    OptionType     [2]byte    // char OptionType[2]
    CALevel        int16      // int16_t
}

// RequestMsg 对应 C++: RequestMsg
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:134-295
// ⚠️ 需要用 offset_check 验证所有字段偏移量
type RequestMsg struct {
    ContractDesc   ContractDescription  // 嵌套结构体
    RequestType    int32     // enum RequestType: NEWORDER=0, MODIFYORDER=1, CANCELORDER=2, ...
    OrdType        int32     // enum OrderType: LIMIT=1, MARKET=2
    Duration       int32     // enum OrderDuration: DAY=0, IOC=1, FOK=2, FAK=4
    PxType         int32     // enum PriceType
    PosDirection   int32     // enum PositionDirection: OPEN=10, CLOSE=11, CLOSE_INTRADAY=12
    OrderID        uint32    // clientId * 1000000 + seq
    Token          int32
    Quantity       int32
    QuantityFilled int32
    DisclosedQnty  int32
    Price          float64
    TimeStamp      uint64
    AccountID      [11]byte  // MAX_ACCNTID_LEN + 1
    TransactionType byte     // 'B' = buy, 'S' = sell
    ExchangeType   byte      // CHINA_SHFE=57, CHINA_CFFEX=58, ...
    Padding        [20]byte
    Product        [32]byte
    StrategyID     int32
}
// 注意: C++ 使用 __attribute__((aligned(64)))
// Go 中通过在 QueueElem 计算中补齐到 64 字节边界
```

### 6.2 ResponseMsg

```go
// ResponseMsg 对应 C++: ResponseMsg
// C++ 源: hftbase/CommonUtils/include/orderresponse.h:436-561
type ResponseMsg struct {
    ResponseType   int32     // enum: NEW_ORDER_CONFIRM=0, TRADE_CONFIRM=4, ORDER_ERROR=5, ...
    ChildResponse  int32     // enum SubResponseType
    OrderID        uint32
    ErrorCode      uint32
    Quantity       int32     // 成交量（仅 TRADE_CONFIRM）
    _pad0          [4]byte   // Quantity(4) 后可能有 padding 到 double 对齐
    Price          float64   // 成交价（仅 TRADE_CONFIRM）
    TimeStamp      uint64
    Side           byte      // 'B' 或 'S'
    Symbol         [50]byte  // MAX_SYMBOL_SIZE = 50（注意：不是 24！）
    AccountID      [11]byte
    _pad1          [4]byte   // 可能的 padding 到 double 对齐
    ExchangeOrderId float64  // 交易所订单ID（NSE 用 double，中国期货用 uint64 union）
    ExchangeTradeId [21]byte // MAX_TRADE_ID_SIZE = 21
    OpenClose       byte     // enum OpenCloseType: OPEN=1, CLOSE=2, CLOSE_TODAY=3
    ExchangeID      byte     // enum TsExchangeID: SHFE=1, INE=2, CZCE=3, DCE=4, CFFEX=5, GFEX=6
    _pad2           [1]byte  // 可能的 padding
    Product         [32]byte
    StrategyID      int32
}
```

**⚠️ 以上 padding 是估算值。必须通过 offset_check 工具精确验证。**

### 6.3 MarketUpdateNew

```go
// BookElement 对应 C++: bookElement_t（继承 orderQtPair_t）
// C++ 源: hftbase/CommonUtils/include/marketupdateNew.h:134-158
type BookElement struct {
    Quantity   int32   // orderQtPair_t::quantity
    OrderCount int32   // orderQtPair_t::orderCount
    Price      float64 // bookElement_t::price
}
// sizeof = 4 + 4 + 8 = 16 bytes

// MarketUpdateNew 对应 C++: struct MarketUpdateNew : MDHeaderPart, MDDataPart
// C++ 源: hftbase/CommonUtils/include/marketupdateNew.h
//
// MDHeaderPart 部分:
//   uint64_t m_exchTS, m_timestamp, m_seqnum, m_rptseqnum, m_tokenId  (5 × 8 = 40)
//   char m_symbol[48]   (MAX_SYMBOL_SIZE - 2 = 48)
//   uint16_t m_symbolID (2)
//   unsigned char m_exchangeName (1)
//   → 合计 91 bytes + padding
//
// MDDataPart 部分:
//   double m_newPrice, m_oldPrice, m_lastTradedPrice (3 × 8 = 24)
//   uint64_t m_lastTradedTime (8)
//   double m_totalTradedValue (8)
//   int64_t m_totalTradedQuantity (8)
//   double m_yield (8)
//   bookElement_t m_bidUpdates[20] (20 × 16 = 320)
//   bookElement_t m_askUpdates[20] (20 × 16 = 320)
//   int32_t m_newQuant, m_oldQuant, m_lastTradedQuantity (3 × 4 = 12)
//   int8_t m_validBids, m_validAsks, m_updateLevel (3)
//   uint8_t m_endPkt (1)
//   unsigned char m_side, m_updateType, m_feedType (3)
//
// 具体 Go 定义需要 offset_check 验证后确定，此处给出框架：
type MarketUpdateNew struct {
    // --- MDHeaderPart ---
    ExchTS       uint64
    Timestamp    uint64
    SeqNum       uint64
    RptSeqNum    uint64
    TokenID      uint64
    Symbol       [48]byte   // MAX_SYMBOL_SIZE - 2
    SymbolID     uint16
    ExchangeName byte       // CHINA_SHFE=57, etc.

    // --- MDDataPart（可能有 padding）---
    // offset_check 验证后填写精确布局
    NewPrice            float64
    OldPrice            float64
    LastTradedPrice     float64
    LastTradedTime      uint64
    TotalTradedValue    float64
    TotalTradedQuantity int64
    Yield               float64
    BidUpdates          [20]BookElement  // 320 bytes
    AskUpdates          [20]BookElement  // 320 bytes
    NewQuant            int32
    OldQuant            int32
    LastTradedQuantity  int32
    ValidBids           int8
    ValidAsks           int8
    UpdateLevel         int8
    EndPkt              uint8
    Side                byte
    UpdateType          byte
    FeedType            byte
}
```

---

## 7. 配置参数（SHM Key）

ORS 的 SHM key 来自配置文件（`.cfg`），Go trader 需读取相同的 key：

```
# 典型 ORS 配置值（示例）
REQUEST_SHMKEY    = 3872
REQUEST_SHMSIZE   = 4096    # 向上取整到 2 的幂
RESPONSE_SHMKEY   = 4872
RESPONSE_SHMSIZE  = 4096
MD_SHMKEY         = 872
MD_SHMSIZE        = 4096
CLIENT_STORE_KEY  = 5872
MAX_CLIENTS       = 250
```

Go trader YAML 配置对应：
```yaml
# tbsrc-golang 配置
shm:
  request_key: 3872
  request_size: 4096
  response_key: 4872
  response_size: 4096
  md_key: 872
  md_size: 4096
  client_store_key: 5872
```

---

## 8. Connector（Go 翻译）

### 8.1 OrderID 生成

```go
// C++ 源: hftbase/CommonUtils/include/constants.h:14
const OrderIDRange int32 = 1_000_000

// C++ 源: hftbase/Connector/include/connector.h:362-367
// GetUniqueOrderNumber(exchCode):
//   return clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++)
func (c *Connector) GetUniqueOrderNumber() uint32 {
    c.OrderCount++
    if c.OrderCount >= OrderIDRange {
        panic("OrderCount overflow — need new clientId")
    }
    return uint32(c.ClientID)*uint32(OrderIDRange) + c.OrderCount
}
```

### 8.2 回报过滤

```go
// C++ 源: Connector::HandleOrderResponse
// clientId = OrderID / ORDERID_RANGE
func (c *Connector) isMyOrder(resp *ResponseMsg) bool {
    return int32(resp.OrderID/uint32(OrderIDRange)) == c.ClientID
}
```

### 8.3 轮询循环

```go
// C++ 源: shmmanager.h — startMonitorAsyncMarketData / startMonitorAsyncORSResponse

func (c *Connector) pollMD() {
    var raw MarketUpdateNew
    for c.running.Load() {
        if !c.mdQueue.IsEmpty() {
            c.mdQueue.Dequeue(&raw)
            c.mdCallback(&raw)
        } else {
            runtime.Gosched()  // 等价于 asm("pause")
        }
    }
}

func (c *Connector) pollORS() {
    var raw ResponseMsg
    for c.running.Load() {
        if !c.respQueue.IsEmpty() {
            c.respQueue.Dequeue(&raw)
            if c.isMyOrder(&raw) {
                c.orsCallback(&raw)
            }
        } else {
            runtime.Gosched()
        }
    }
}
```

---

## 9. Offset Check 验证工具

### 9.1 C++ 版（编译到 hftbase 环境）

```cpp
// gateway/tools/offset_check.cpp
#include "orderresponse.h"
#include "marketupdateNew.h"
#include "multiwritermultireadershmqueue.h"
#include <cstdio>
#include <cstddef>

using namespace illuminati;

int main() {
    printf("=== RequestMsg ===\n");
    printf("sizeof=%zu\n", sizeof(infra::RequestMsg));
    printf("  Contract_Description offset=%zu\n", offsetof(infra::RequestMsg, Contract_Description));
    printf("  Request_Type         offset=%zu\n", offsetof(infra::RequestMsg, Request_Type));
    printf("  OrdType              offset=%zu\n", offsetof(infra::RequestMsg, OrdType));
    printf("  OrderID              offset=%zu\n", offsetof(infra::RequestMsg, OrderID));
    printf("  Price                offset=%zu\n", offsetof(infra::RequestMsg, Price));
    printf("  TimeStamp            offset=%zu\n", offsetof(infra::RequestMsg, TimeStamp));
    printf("  AccountID            offset=%zu\n", offsetof(infra::RequestMsg, AccountID));
    printf("  Transaction_Type     offset=%zu\n", offsetof(infra::RequestMsg, Transaction_Type));
    printf("  Exchange_Type        offset=%zu\n", offsetof(infra::RequestMsg, Exchange_Type));
    printf("  Product              offset=%zu\n", offsetof(infra::RequestMsg, Product));
    printf("  StrategyID           offset=%zu\n", offsetof(infra::RequestMsg, StrategyID));

    printf("\n=== ResponseMsg ===\n");
    printf("sizeof=%zu\n", sizeof(infra::ResponseMsg));
    printf("  Response_Type   offset=%zu\n", offsetof(infra::ResponseMsg, Response_Type));
    printf("  OrderID         offset=%zu\n", offsetof(infra::ResponseMsg, OrderID));
    printf("  Quantity        offset=%zu\n", offsetof(infra::ResponseMsg, Quantity));
    printf("  Price           offset=%zu\n", offsetof(infra::ResponseMsg, Price));
    printf("  Side            offset=%zu\n", offsetof(infra::ResponseMsg, Side));
    printf("  Symbol          offset=%zu\n", offsetof(infra::ResponseMsg, Symbol));
    printf("  AccountID       offset=%zu\n", offsetof(infra::ResponseMsg, AccountID));
    printf("  ExchangeOrderId offset=%zu\n", offsetof(infra::ResponseMsg, ExchangeOrderId));
    printf("  ExchangeTradeId offset=%zu\n", offsetof(infra::ResponseMsg, ExchangeTradeId));
    printf("  OpenClose       offset=%zu\n", offsetof(infra::ResponseMsg, OpenClose));
    printf("  ExchangeID      offset=%zu\n", offsetof(infra::ResponseMsg, ExchangeID));
    printf("  Product         offset=%zu\n", offsetof(infra::ResponseMsg, Product));
    printf("  StrategyID      offset=%zu\n", offsetof(infra::ResponseMsg, StrategyID));

    printf("\n=== MarketUpdateNew ===\n");
    printf("sizeof=%zu\n", sizeof(md::MarketUpdateNew));
    // ... 所有字段

    printf("\n=== QueueElem<RequestMsg> ===\n");
    printf("sizeof=%zu\n", sizeof(ds::QueueElem<infra::RequestMsg>));
    printf("  data   offset=%zu\n", offsetof(ds::QueueElem<infra::RequestMsg>, data));
    printf("  seqNo  offset=%zu\n", offsetof(ds::QueueElem<infra::RequestMsg>, seqNo));

    printf("\n=== QueueElem<ResponseMsg> ===\n");
    printf("sizeof=%zu\n", sizeof(ds::QueueElem<infra::ResponseMsg>));
    printf("  data   offset=%zu\n", offsetof(ds::QueueElem<infra::ResponseMsg>, data));
    printf("  seqNo  offset=%zu\n", offsetof(ds::QueueElem<infra::ResponseMsg>, seqNo));

    printf("\n=== QueueElem<MarketUpdateNew> ===\n");
    printf("sizeof=%zu\n", sizeof(ds::QueueElem<md::MarketUpdateNew>));
    printf("  data   offset=%zu\n", offsetof(ds::QueueElem<md::MarketUpdateNew>, data));
    printf("  seqNo  offset=%zu\n", offsetof(ds::QueueElem<md::MarketUpdateNew>, seqNo));

    printf("\n=== MultiWriterMultiReaderShmHeader ===\n");
    printf("sizeof=%zu\n", sizeof(ds::MultiWriterMultiReaderShmHeader));

    return 0;
}
```

### 9.2 Go 版

```go
// cmd/offset_check/main.go
package main

import (
    "fmt"
    "unsafe"
    "tbsrc-golang/pkg/shm"
)

func main() {
    var req shm.RequestMsg
    fmt.Printf("=== RequestMsg ===\n")
    fmt.Printf("sizeof=%d\n", unsafe.Sizeof(req))
    fmt.Printf("  ContractDesc     offset=%d\n", unsafe.Offsetof(req.ContractDesc))
    fmt.Printf("  RequestType      offset=%d\n", unsafe.Offsetof(req.RequestType))
    // ... 所有字段

    var resp shm.ResponseMsg
    fmt.Printf("\n=== ResponseMsg ===\n")
    fmt.Printf("sizeof=%d\n", unsafe.Sizeof(resp))
    // ... 所有字段

    var md shm.MarketUpdateNew
    fmt.Printf("\n=== MarketUpdateNew ===\n")
    fmt.Printf("sizeof=%d\n", unsafe.Sizeof(md))
    // ... 所有字段
}
```

### 9.3 验证流程

```bash
# 1. 编译 C++ offset_check（在 hftbase 环境中）
cd /Users/user/PWorks/RD/hftbase
g++ -std=c++17 -I CommonUtils/include -I Ipc/include -I Logger/include \
    ../quantlink-trade-system/tbsrc-golang/tools/offset_check.cpp \
    -o /tmp/cpp_offset_check
/tmp/cpp_offset_check > /tmp/cpp_offsets.txt

# 2. 运行 Go offset_check
cd /Users/user/PWorks/RD/quantlink-trade-system/tbsrc-golang
go run cmd/offset_check/main.go > /tmp/go_offsets.txt

# 3. 对比（必须完全一致）
diff /tmp/cpp_offsets.txt /tmp/go_offsets.txt
```

---

## 10. 与 counter_bridge 的关系

### 10.1 架构选择

Go trader **不再对接** counter_bridge（新 gateway SPSC 队列 + `OrderRequestRaw`/`OrderResponseRaw`），而是**直接对接原 ORS**（hftbase MWMR 队列 + `RequestMsg`/`ResponseMsg`）。

```
原架构（golang/ 旧代码）:
  trader → [gRPC] → ors_gateway → [SPSC SHM] → counter_bridge → CTP

新架构（tbsrc-golang）:
  go_trader → [MWMR SHM, SysV] → ORS (Shengli/China/...) → 交易所
  ↑                                    ↓
  └──────── [MWMR SHM, SysV] ←────────┘
```

### 10.2 收益

| 收益 | 说明 |
|------|------|
| RMS 风控 | ORS 内建的 RmsImpl：订单限额、自成交检查、撤单次数限制 |
| 开平推断 | ORS 的 `SetCombOffsetFlag()` 自动根据持仓决定平今/平昨/开仓 |
| 多账户 | ORS 的 `m_accountMap[clientId]` 自动填充 AccountID |
| 持仓跟踪 | ORS 的 `mapContractPos` 实时跟踪各合约持仓 |
| 生产验证 | hftbase MWMR 已在生产环境长期运行 |
| 多 trader | 天然支持多个 trader 进程共享队列 |

### 10.3 counter_bridge 不需要修改

不再需要将 counter_bridge 从 SPSC 改为 MWMR。counter_bridge 仅供旧 golang/ 代码使用，tbsrc-golang 直接绕过它。

---

## 11. 注意事项和风险

### 11.1 ⚠️ enum 大小

C++ enum 默认是 `int`（4 bytes），Go 中需要用 `int32` 对应。确认：
- `RequestType` → int32
- `OrderType` → int32
- `OrderDuration` → int32
- `PriceType` → int32
- `PositionDirection` → int32
- `ResponseType` → int32
- `SubResponseType` → int32

`OpenCloseType` 和 `TsExchangeID` 是 `enum class : char`（1 byte），Go 中用 `byte`。

### 11.2 ⚠️ `__attribute__((aligned(64)))`

`RequestMsg` 有 cache-line 对齐属性。这影响 `sizeof(RequestMsg)` 和 `sizeof(QueueElem<RequestMsg>)`。Go struct 没有对齐属性，需要手动在末尾添加 padding 使 `unsafe.Sizeof(RequestMsg{})` 等于 C++ 的 `sizeof(RequestMsg)`。

### 11.3 ⚠️ head 初始值

C++ MWMR 的 `head` 初始值是 **1**，`tail` 初始值也是 **1**。Go 的 `localTail` 初始化也必须是 1。

如果 Go 进程在 ORS 已经运行一段时间后才启动，`localTail` 应初始化为当前 `head` 值（追上最新位置），否则会读到过期数据。这与 C++ 的行为一致：

```cpp
// multiwritermultireadershmqueue.h:99
tail = ShmStore::header->head.load(std::memory_order_relaxed);
```

### 11.4 ⚠️ SHM 页面对齐

SHM 大小按系统页面大小（通常 4096）向上对齐：

```go
pageSize := syscall.Getpagesize()
shmSize := headerSize + size*elemSize
shmSize = shmSize + pageSize - (shmSize % pageSize)
```

---

## 12. 实现顺序

1. **SysV SHM 包** — `shmget`/`shmat`/`shmdt` Go 封装（~30 行）
2. **Struct 定义** — `RequestMsg`/`ResponseMsg`/`MarketUpdateNew`/`BookElement`（~80 行）
3. **Offset Check 工具** — Go + C++ 对比验证（各 ~50 行）
4. **运行 Offset Check** — 确保所有字段偏移量和 sizeof 完全一致
5. **MWMR Queue** — `Enqueue`/`Dequeue`/`IsEmpty`（~60 行）
6. **ClientStore** — `GetClientIDAndIncrement`（~15 行）
7. **MWMR 单元测试** — Go 进程间 SHM 读写测试
8. **C++ 互操作测试** — Go writer → C++ reader / C++ writer → Go reader

---

## 参考资料

- hftbase MWMR 队列: `hftbase/Ipc/include/multiwritermultireadershmqueue.h`
- SHM 内存管理: `hftbase/Ipc/include/shmallocator.h` + `sharedmemory.h`
- ClientStore: `hftbase/Ipc/include/locklessshmclientstore.h`
- ShmManager: `hftbase/Ipc/include/shmmanager.h`
- 消息定义: `hftbase/CommonUtils/include/orderresponse.h`
- 行情定义: `hftbase/CommonUtils/include/marketupdateNew.h`
- 常量: `hftbase/CommonUtils/include/constants.h`
- ORS 实现（盛立）: `ors/Shengli/include/ORSServer.h` + `src/ORSServer.cpp`
- ORS 实现（CTP）: `ors/China/include/ORSServer.h` + `src/ORSServer.cpp`
- Connector: `hftbase/Connector/include/connector.h`

---

**最后更新**: 2026-02-13
