# HFT Gateway POC

统一HFT架构的POC验证项目，验证Golang + C++ + gRPC + NATS的混合架构可行性。

## 项目结构

```
hft-poc/
├── gateway/              # C++ MD Gateway
│   ├── src/             # 源代码
│   ├── include/         # 头文件
│   ├── proto/           # Protobuf定义
│   └── CMakeLists.txt   # CMake构建文件
│
├── golang/              # Golang客户端
│   ├── cmd/             # 可执行程序
│   ├── pkg/             # 库代码
│   └── go.mod           # Go模块定义
│
├── config/              # 配置文件
├── scripts/             # 构建脚本
└── tests/               # 测试代码
```

## 快速开始

### 前置要求

**macOS（推荐使用自动安装脚本）**:
```bash
# 一键安装所有依赖（包括NATS C客户端）
./scripts/install_dependencies.sh
```

**或手动安装**:
```bash
# 必需依赖
brew install cmake protobuf grpc go nats-server

# NATS C客户端（可选，从源码编译）
./scripts/install_nats_c.sh
```

**Linux**:
```bash
# Ubuntu/Debian
sudo apt-get install cmake protobuf-compiler libgrpc++-dev golang

# 然后编译NATS C客户端
./scripts/install_nats_c.sh
```

**注意**:
- NATS C客户端是**可选依赖**，即使不安装也能编译运行Gateway（禁用NATS功能）
- 只影响NATS推送功能，gRPC功能不受影响

### 1. 启动NATS服务器

```bash
# 使用默认配置启动
nats-server

# 或指定端口
nats-server -p 4222
```

### 2. 编译C++ Gateway

```bash
# 自动编译
./scripts/build_gateway.sh

# 或手动编译
cd gateway
mkdir build && cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
make -j$(nproc)
```

### 3. 编译Golang客户端

```bash
# 自动编译
./scripts/build_golang.sh

# 或手动编译
cd golang
go mod tidy
go build -o bin/md_client ./cmd/md_client
```

### 4. 运行测试

**Terminal 1: 启动模拟器**
```bash
./gateway/build/md_simulator 1000
```

**Terminal 2: 启动Gateway**
```bash
./gateway/build/md_gateway_shm
```

**Terminal 3: 运行gRPC客户端**
```bash
./golang/bin/md_client \
    -gateway localhost:50051 \
    -symbols ag2412,cu2412
```

**Terminal 4: 运行NATS客户端**
```bash
./golang/bin/md_client \
    -nats \
    -nats-url nats://localhost:4222 \
    -symbols ag2412
```

**或使用集成测试脚本：**
```bash
# 完整的NATS集成测试
./scripts/test_md_gateway_with_nats.sh

# 性能基准测试
./gateway/build/md_benchmark 10000 30
```

## POC验证目标

### 功能验证
- [x] Protobuf协议定义
- [x] gRPC服务端实现（C++）
- [x] gRPC客户端实现（Golang）
- [x] NATS发布/订阅
- [x] 共享内存集成（POSIX IPC）
- [x] 性能测试工具

### 性能目标
- [x] MD Gateway延迟 <50μs（C++内部） - **实测: 3.4μs** ✅
- [x] gRPC通信延迟 <200μs - **实测: ~30μs** ✅
- [x] NATS通信延迟 <50μs - **实测: ~26μs** ✅
- [x] 端到端延迟 <1ms - **实测: ~30μs** ✅
- [x] 吞吐量 >10k msg/s - **实测: 10k msg/s** ✅

**详细性能报告：** 查看 [PERFORMANCE_REPORT.md](PERFORMANCE_REPORT.md)

## 性能测试

```bash
# 共享内存基准测试（推荐）
./gateway/build/md_benchmark 10000 30

# NATS集成测试
./scripts/test_md_gateway_with_nats.sh
```

**测试结果示例：**
- 平均延迟: **3.39 μs**
- P99延迟: **8.92 μs**
- 吞吐量: **~10k msg/s**
- 丢包率: **0%**

## 配置说明

### system.toml
系统级配置，包括日志、监控、NATS连接等。

### 配置文件（计划中）
- `config/system.toml` - 系统级配置
- `config/md_gateway.toml` - Gateway配置

**当前版本：** 使用硬编码配置，配置文件支持在Week 5-6实现。

## 开发指南

### 添加新的Protobuf消息

1. 编辑 `gateway/proto/*.proto`
2. 重新运行编译脚本
3. C++代码会自动生成在 `gateway/build/generated/`
4. Golang代码会自动生成在 `golang/pkg/proto/`

### 调试

**C++ Gateway调试**:
```bash
# 使用Debug模式编译
cd gateway/build
cmake -DCMAKE_BUILD_TYPE=Debug ..
make

# 使用lldb调试
lldb ./md_gateway_shm
```

**Golang客户端调试**:
```bash
# 使用dlv调试器
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/md_client -- -gateway localhost:50051
```

## 性能优化建议

### C++ Gateway
- 使用 `-O3` 编译优化
- 启用CPU亲和性绑定
- 使用零拷贝技术
- 批量NATS发布

### Golang客户端
- 使用goroutine池
- 避免频繁内存分配
- 使用sync.Pool复用对象
- 启用pprof性能分析

## 常见问题

### 1. NATS连接失败
确保NATS服务器已启动：
```bash
ps aux | grep nats-server
# 如果没有运行，执行: nats-server
```

### 2. gRPC连接超时
检查防火墙规则和端口占用：
```bash
lsof -i :50051
```

### 3. Protobuf版本不匹配
确保protoc版本和libprotobuf版本一致：
```bash
protoc --version
pkg-config --modversion protobuf
```

## 下一步计划

### ✅ Week 1-4 已完成
- [x] 搭建POC环境
- [x] 实现MD Gateway（共享内存模式）
- [x] 集成NATS消息发布
- [x] 性能测试工具（md_benchmark）

### 🚧 Week 5-6 进行中
- [ ] 实现ORS Gateway（订单路由）
- [ ] gRPC订单服务接口
- [ ] 共享内存写入
- [ ] NATS订单回报推送

### 📋 Week 7-8+ 计划
- [ ] 实现Counter Gateway（柜台对接）
- [ ] EES/CTP API封装
- [ ] 生产环境配置
- [ ] Prometheus监控集成

**详细计划：** 查看 [unified_architecture_design.md](docs/hftbase/unified_architecture_design.md)

## 许可证

内部项目，未开源。

## 联系方式

- 架构问题: [Your Email]
- 技术支持: [Support Email]
