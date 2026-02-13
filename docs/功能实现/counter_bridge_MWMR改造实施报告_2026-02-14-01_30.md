# Counter Bridge MWMR 改造实施报告

**文档日期**: 2026-02-14
**版本**: v1.0
**相关模块**: gateway/src/counter_bridge.cpp, gateway/include/hftbase_shm.h, gateway/include/hftbase_types.h

---

## 概述

将 counter_bridge 从 POSIX SPSC + OrderRequestRaw/OrderResponseRaw 改造为 SysV MWMR + RequestMsg/ResponseMsg，实现与 tbsrc-golang (Go trader) 和 hftbase (原始 C++ ORS) 的二进制兼容。

这是 Go 策略层进入实盘的核心前提——Go trader 通过 SysV 共享内存直接与 counter_bridge 交互，不再经过 gRPC/ors_gateway 中转。

## 改造内容

### 1. 新增 hftbase_shm.h — SysV MWMR 队列实现

**文件**: `gateway/include/hftbase_shm.h`

| 组件 | 说明 | 兼容目标 |
|------|------|---------|
| `shm_create()` / `shm_open_existing()` | SysV 共享内存创建/打开 | hftbase/Ipc/include/sharedmemory.h |
| `MWMRQueue<T>` | 多写多读无锁队列模板 | hftbase/Ipc/include/multiwritermultireadershmqueue.h |
| `MWMRHeader` | 8 字节原子头 (head=1 初始值) | MultiWriterMultiReaderShmHeader |
| `QueueElem<T>` | data + seqNo 包装 | QueueElem<T> |
| `ClientStore` | 原子计数器 (orderID 生成) | hftbase/Ipc/include/locklessshmclientstore.h |

**内存布局**:
```
[MWMRHeader: 8 bytes] [QueueElem[0]] [QueueElem[1]] ... [QueueElem[size-1]]
                       ↑ data + seqNo
```

- 队列大小向上取整到 2 的幂次（与 hftbase 一致）
- 入队: `fetch_add(head)` → 写入 slot → 设置 seqNo
- 出队: 检查 `slot->seqNo >= tail` → 复制数据 → tail = seqNo + 1

### 2. 新增 hftbase_types.h — RequestMsg/ResponseMsg 结构体

**文件**: `gateway/include/hftbase_types.h`

三方二进制兼容验证:
```
hftbase/CommonUtils/include/orderresponse.h  ←→  hftbase_types.h  ←→  tbsrc-golang/pkg/shm/types.go
```

| 结构体 | 大小 | 对齐 | static_assert |
|--------|------|------|--------------|
| ContractDescription | 96 bytes | 自然 | ✅ |
| RequestMsg | 256 bytes | aligned(64) | ✅ |
| ResponseMsg | 176 bytes | 自然 | ✅ |

**枚举定义** (值与 C++ 完全一致):
- RequestType: NEWORDER=0, MODIFYORDER=1, CANCELORDER=2 ...
- ResponseType: NEW_ORDER_CONFIRM=0, TRADE_CONFIRM=4, ORDER_ERROR=5 ...
- SubResponseType, PositionDirection, OrderType, OrderDuration, PriceType
- OpenCloseType (char enum): OCT_OPEN=1, OCT_CLOSE=2, OCT_CLOSE_TODAY=3
- TsExchangeID (char enum): TSEXCH_SHFE=1, TSEXCH_CFFEX=5 ...
- Exchange_Type 字节常量: CHINA_SHFE=57, CHINA_CFFEX=58 ...

### 3. 改造 counter_bridge.cpp

**文件**: `gateway/src/counter_bridge.cpp`

#### 移除的内容
- `#include "shm_queue.h"` (POSIX SPSC)
- `#include "ors_gateway.h"` (OrderRequestRaw/OrderResponseRaw)
- gRPC/protobuf 依赖
- HTTP `/positions` 端点

#### 新增的内容

| 功能 | C++ 参考 | 说明 |
|------|---------|------|
| `SetCombOffsetFlag()` | ors/China/src/ORSServer.cpp:488-605 | 开平自动推断 |
| `updatePosition()` | ors/China/src/ORSServer.cpp:1186-1281 | 持仓跟踪 |
| `loadPositionFile()` | — | 初始持仓加载 (CSV 格式) |
| `contractPos` 结构 | ors/Shengli/include/ORSServer.h:102-108 | 隔夜/今仓四元组 |

#### SetCombOffsetFlag 逻辑

```
买入:
  1. 今空仓 >= 数量 → CLOSE_TODAY (SHFE) / CLOSE_YESTD (其他)
  2. 隔夜空仓 >= 数量 → CLOSE_YESTD
  3. 否则 → OPEN

卖出:
  1. 今多仓 >= 数量 → CLOSE_TODAY (SHFE) / CLOSE_YESTD (其他)
  2. 隔夜多仓 >= 数量 → CLOSE_YESTD
  3. 否则 → OPEN
```

SHFE/INE 区分平今/平昨，其他交易所统一使用 CLOSE_YESTD。

#### updatePosition 逻辑

| 事件 | 操作 |
|------|------|
| TRADE_CONFIRM + OPEN_ORDER | 增加今仓 (buy→todayLong, sell→todayShort) |
| TRADE_CONFIRM + CLOSE | 无操作 (SetCombOffsetFlag 已扣减) |
| REJECT/CANCEL + CLOSE_TODAY | 恢复今仓 |
| REJECT/CANCEL + CLOSE_YESTD | 恢复隔夜仓 |
| REJECT/CANCEL + OPEN_ORDER | 无操作 |

#### SHM 配置

| 参数 | SysV Key | 默认大小 |
|------|---------|---------|
| 请求队列 | 0x2001 | 4096 |
| 响应队列 | 0x3001 | 4096 |
| ClientStore | 0x4001 | — |

与 Go `trader.tbsrc.yaml` 中的 `req_shm_key`/`resp_shm_key`/`client_store_shm_key` 一致。

### 4. CMakeLists.txt 更新

counter_bridge 不再链接 gRPC/protobuf:
- 移除 `${PROTO_SRCS}`, `${GRPC_SRCS}` 源文件
- 移除 `${GENERATED_PROTOBUF_PATH}` 包含目录
- 移除 `gRPC::grpc++`, `gRPC::grpc++_reflection` 链接库

仅保留: Threads, yaml-cpp, 插件源文件 (CTP/Simulator)

## 架构变更

### 改造前

```
Go trader → [gRPC] → ors_gateway → [POSIX SHM SPSC] → counter_bridge → CTP/Simulator
                                  ← [POSIX SHM SPSC] ←
```

OrderRequestRaw/OrderResponseRaw (自定义结构，与 hftbase 不兼容)

### 改造后

```
Go trader → [SysV MWMR SHM] → counter_bridge → CTP/Simulator
          ← [SysV MWMR SHM] ←
```

RequestMsg/ResponseMsg (256/176 bytes，hftbase 二进制兼容)

**消除的中间层**: ors_gateway (gRPC 服务端) 不再需要用于 counter_bridge 通路

## 修改文件

| 文件 | 变更 |
|------|------|
| `gateway/include/hftbase_shm.h` | 新增：SysV MWMR 队列 + ClientStore |
| `gateway/include/hftbase_types.h` | 新增：RequestMsg/ResponseMsg + 枚举 |
| `gateway/src/counter_bridge.cpp` | 重写：MWMR + SetCombOffsetFlag |
| `gateway/CMakeLists.txt` | 修改：移除 gRPC/protobuf 依赖 |

## 验证

- counter_bridge 编译通过 (macOS, Release)
- 181 个 Go 测试全部通过 (8 包)
- Linux/amd64 交叉编译通过
- static_assert 验证 RequestMsg=256, ResponseMsg=176, ContractDescription=96

## 向后兼容

- `gateway/include/shm_queue.h` (POSIX SPSC) 保留，供 ors_gateway 等旧代码使用
- `gateway/include/ors_gateway.h` (OrderRequestRaw/OrderResponseRaw) 保留
- ors_gateway 本身不受影响，仍然可用于其他通路

## 剩余工作

| 任务 | 优先级 | 说明 |
|------|--------|------|
| Go connector SysV MWMR 适配 | P0 | Go 端使用 SysV MWMR 替代 gRPC |
| 端到端集成测试 | P0 | Go trader ↔ counter_bridge 全链路验证 |
| Linux 交叉编译 counter_bridge | P1 | 生产环境部署 |
| INE 交易所平今/平昨支持 | P2 | 当前仅 SHFE 区分 |

---

**最后更新**: 2026-02-14 01:30
