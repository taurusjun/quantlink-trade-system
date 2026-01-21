# HFT POC Project Overview

## 项目概览

**创建时间**: 2026-01-19
**目标**: 验证统一HFT架构的技术可行性
**周期**: Week 1-2 (POC阶段)

## 生成的代码

### C++ Gateway (~2000行)
```
gateway/
├── include/
│   ├── md_gateway.h                  # MD Gateway头文件 (~200行)
│   ├── shm_queue.h                   # 共享内存队列 (~160行)
│   └── performance_monitor.h         # 性能监控 (~180行)
├── src/
│   ├── md_gateway.cpp                # MD Gateway实现 (~360行)
│   ├── main_shm.cpp                  # Gateway主程序-共享内存 (~170行)
│   ├── md_simulator.cpp              # 行情模拟器 (~200行)
│   └── md_benchmark.cpp              # 性能基准测试 (~400行)
├── proto/
│   ├── market_data.proto             # 行情协议定义
│   └── common.proto                  # 通用协议定义
└── CMakeLists.txt                    # CMake构建配置 (~130行)
```

### Golang Client (~600行)
```
golang/
├── cmd/md_client/
│   └── main.go                       # 客户端入口 (~300行)
├── pkg/client/
│   └── md_client.go                  # 客户端库 (~300行)
└── go.mod                            # Go模块定义
```

### 配置和脚本 (~400行)
```
config/
├── system.toml                       # 系统配置
└── md_gateway.toml                   # Gateway配置

scripts/
├── build_gateway.sh                  # C++编译脚本
├── build_golang.sh                   # Go编译脚本
└── run_test.sh                       # 集成测试脚本
```

## 架构验证点

### ✅ 已完成
1. **Protobuf协议设计**
   - 行情数据结构定义
   - gRPC服务接口定义
   - 支持流式推送

2. **C++ Gateway实现**
   - gRPC服务端
   - NATS发布者
   - 订单簿缓存
   - 并发安全

3. **Golang客户端实现**
   - gRPC客户端
   - NATS订阅者
   - 统一数据结构
   - 性能统计

4. **构建系统**
   - CMake自动化构建
   - Protobuf代码生成
   - Go模块管理

### ✅ Week 3-4 已验证
1. **性能指标**
   - [x] 共享内存延迟 <10μs - **实测: 3.4μs** ✅
   - [x] NATS延迟 <50μs - **实测: ~26μs** ✅
   - [x] 端到端 <1ms - **实测: ~30μs** ✅
   - [x] 吞吐量 >10k msg/s - **实测: ~10k msg/s** ✅

2. **共享内存集成**
   - [x] POSIX共享内存（shm_open/mmap）
   - [x] 零拷贝读取（SPSC无锁队列）
   - [x] 性能对比完成（查看PERFORMANCE_REPORT.md）

3. **测试工具**
   - [x] 性能基准测试工具（md_benchmark）
   - [x] NATS集成测试脚本
   - [x] 详细性能监控

### 🔄 Week 5+ 待实现
1. **ORS Gateway**
   - [ ] 订单路由gRPC接口
   - [ ] 共享内存订单队列
   - [ ] 订单回报推送

2. **Counter Gateway**
   - [ ] Counter抽象接口
   - [ ] EES/CTP API封装
   - [ ] 订单映射管理

## 技术栈

| 组件 | 技术 | 版本 |
|-----|------|------|
| C++ Gateway | C++17 | - |
| Golang Client | Go | 1.21+ |
| RPC | gRPC | 1.60+ |
| Messaging | NATS | 2.10+ |
| Serialization | Protobuf | 3.21+ |
| Build | CMake | 3.15+ |

## 下一步行动

### Week 1-4 ✅ 已完成
- [x] 生成POC代码
- [x] 编译和运行验证
- [x] 修复编译错误
- [x] 基础功能测试
- [x] 性能基准测试
- [x] 共享内存集成（POSIX IPC）
- [x] 延迟优化（P99 <9μs）
- [x] 文档完善

### Week 5-6 ✅ 已完成
- [x] 实现ORS Gateway
- [x] 订单路由gRPC服务（SendOrder/CancelOrder/QueryOrders）
- [x] 共享内存订单队列（请求/响应）
- [x] 订单回报NATS推送
- [x] 订单簿管理和统计

### Week 7-8 🚧 进行中
- [ ] Golang订单客户端
- [ ] Counter Gateway实现
- [ ] EES/CTP API封装
- [ ] Prometheus监控集成
- [ ] 生产环境配置

## 快速命令

```bash
# 编译C++ Gateway
./scripts/build_gateway.sh

# 编译Golang客户端
./scripts/build_golang.sh

# 启动模拟器
./gateway/build/md_simulator 1000

# 启动Gateway（共享内存模式）
./gateway/build/md_gateway

# 运行性能基准测试
./gateway/build/md_benchmark 10000 30

# 运行NATS集成测试
./scripts/test_md_gateway_with_nats.sh

# 运行客户端（gRPC）
./golang/bin/md_client -gateway localhost:50051 -symbols ag2412

# 运行客户端（NATS）
./golang/bin/md_client -nats -symbols ag2412
```

## 性能目标

| 指标 | 目标值 | 实测值 | 状态 |
|-----|--------|--------|------|
| 共享内存IPC | <50μs | **3.4μs** | ✅ |
| MD Gateway处理 | <50μs | **~30μs** | ✅ |
| NATS发布 | <50μs | **~26μs** | ✅ |
| P99延迟 | <20μs | **8.92μs** | ✅ |
| 吞吐量 | >10k/s | **~10k msg/s** | ✅ |

**详细报告：** [PERFORMANCE_REPORT.md](PERFORMANCE_REPORT.md)

## 文件清单

### 核心代码
- C++源文件: 4个（~1130行）
  - main_shm.cpp, md_gateway.cpp, md_simulator.cpp, md_benchmark.cpp
- C++头文件: 3个（~540行）
  - md_gateway.h, shm_queue.h, performance_monitor.h
- Protobuf定义: 2个
- Golang源文件: 2个（~600行）

### 配置和脚本
- 构建脚本: 3个
- 测试脚本: 2个
- 配置文件: 2个（计划中）

### 文档
- 技术文档: 6个
  - README.md, QUICKSTART.md, USAGE.md
  - SHM_EXAMPLE.md, PERFORMANCE_REPORT.md, CLEANUP_SUMMARY.md

**总代码行数: ~2900行**
