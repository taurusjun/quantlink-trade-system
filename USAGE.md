# HFT Gateway POC - 使用指南

## 📊 系统状态

### ✅ 已完成功能

- **C++ MD Gateway**：成功编译并运行（513 KB）
  - gRPC服务端：监听 0.0.0.0:50051
  - 模拟行情推送：100 msg/s
  - NATS支持：可选（需手动安装）

- **Golang客户端**：成功编译并运行（16 MB）
  - gRPC客户端：实时订阅行情
  - NATS客户端：实时订阅行情（需NATS启用）
  - 性能统计：延迟、吞吐量监控

### 📈 性能指标

| 指标 | 目标 | 实测 | 状态 |
|------|------|------|------|
| gRPC延迟 | <200µs | ~235µs | ⚠️ 接近 |
| 端到端延迟 | <1ms | ~235µs | ✅ 优秀 |
| 吞吐量 | >1000 msg/s | 85 msg/s* | ℹ️ 受限 |

*受限于Gateway模拟器的10ms推送间隔，实际生产环境可达更高

## 🚀 快速启动

### 方式1：共享内存模式（推荐，生产环境）

**Terminal 1 - 启动模拟器：**
```bash
cd /Users/user/PWorks/RD/hft-poc
./gateway/build/md_simulator 1000
```

**Terminal 2 - 启动Gateway：**
```bash
./gateway/build/md_gateway
```

输出示例：
```
╔═══════════════════════════════════════════════════════╗
║    HFT Market Data Gateway - Shared Memory Mode      ║
╚═══════════════════════════════════════════════════════╝

[Main] Opening shared memory: queue
[Main] Shared memory opened successfully
[MDGateway] Connected to NATS: nats://localhost:4222
[Reader] Shared memory reader thread started
[MDGateway] Started successfully
[MDGateway] NATS: Enabled
[MDGateway] gRPC server listening on 0.0.0.0:50051
```

**Terminal 3 - 运行gRPC客户端：**
```bash
./golang/bin/md_client -gateway localhost:50051 -symbols ag2412
```

输出示例：
```
[Client] Connected to gateway: localhost:50051
[Client] Subscribed to symbols: [ag2412]
[Client] Count: 1, Avg Latency: 2.586ms, Throughput: 2655 msg/s
[Client] Count: 2, Avg Latency: 1.3825ms, Throughput: 191 msg/s
[Client] Count: 10, Avg Latency: 400.5µs, Throughput: 98 msg/s
[Client] Count: 20, Avg Latency: 275.25µs, Throughput: 91 msg/s
...
[Client] Count: 1000, Avg Latency: 235µs, Throughput: 85 msg/s
```

### 方式2：一键集成测试（自动化）

**完整NATS集成测试：**
```bash
./scripts/test_md_gateway_with_nats.sh
```

这个脚本会自动：
1. 启动NATS服务器
2. 启动NATS订阅者
3. 启动模拟器
4. 启动Gateway
5. 运行10秒测试
6. 显示结果并清理

**性能基准测试：**
```bash
# 10k Hz频率，持续30秒
./gateway/build/md_benchmark 10000 30
```

## 📝 客户端参数说明

### gRPC模式
```bash
./golang/bin/md_client \
    -gateway localhost:50051 \    # Gateway地址
    -symbols ag2412,cu2412        # 订阅品种（逗号分隔）
```

### NATS模式
```bash
./golang/bin/md_client \
    -nats \                           # 使用NATS模式
    -nats-url nats://localhost:4222 \ # NATS服务器地址
    -symbols ag2412                   # 订阅品种
```

## 🔍 输出说明

### Gateway输出格式

**启动信息：**
```
[MDGateway] Connected to NATS: nats://localhost:4222
[MDGateway] Started successfully
[MDGateway] NATS: Enabled
[MDGateway] gRPC server listening on 0.0.0.0:50051
```

**运行统计：**
```
[MDGateway] Published 1000 messages to NATS (latest: md.SHFE.ag2412)
[MDGateway] Processed 10000 updates, last latency: 29500 ns
[Reader] Read: 10000, Missing: 0, Queue Size: 0, Rate: 1275 msg/s
```

### gRPC客户端输出格式

**统计信息**（每10条打印一次）：
```
[Client] Count: 100, Avg Latency: 235µs, Throughput: 85 msg/s
```
- Count: 已接收消息数量
- Avg Latency: 平均延迟（发送时间戳到接收时间）
- Throughput: 吞吐量（消息数/秒）

**详细行情**（每1000条打印一次）：
```
─────────────────────────────────────
Symbol:    ag2412
Exchange:  SHFE
Timestamp: 2026-01-20 10:28:58.99906 +0800 CST
─────────────────────────────────────
Bid5: 7946.0 × 30  |  Ask5: 7955.0 × 32
Bid4: 7947.0 × 25  |  Ask4: 7954.0 × 27
Bid3: 7948.0 × 20  |  Ask3: 7953.0 × 22
Bid2: 7949.0 × 15  |  Ask2: 7952.0 × 17
Bid1: 7950.0 × 10  |  Ask1: 7951.0 × 12
─────────────────────────────────────
Last: 7950.5 × 5, Volume: 123456
─────────────────────────────────────
```

### NATS客户端输出格式

**实时行情**（每条都打印）：
```
[Client] Received ag2412: BidPx=7950.0, AskPx=7951.0, Latency=156µs
```

## 🛠️ 故障排查

### 问题1：Gateway无法启动 "Failed to open shared memory"

**原因**：模拟器未启动或共享内存不存在
**解决**：
```bash
# 确保先启动模拟器
./gateway/build/md_simulator 1000

# 然后再启动Gateway
./gateway/build/md_gateway
```

### 问题2：NATS未收到消息

**原因**：NATS服务器未启动或连接失败
**解决**：
```bash
# 检查NATS服务器
ps aux | grep nats-server

# 启动NATS服务器
nats-server

# 重新编译Gateway（确保NATS已启用）
./scripts/build_gateway.sh
```

### 问题3：消息丢失（Missing > 0）

**原因**：生产速度大于消费速度
**解决**：
```bash
# 降低模拟器频率
./gateway/build/md_simulator 5000  # 从10k降到5k

# 或增大队列容量（修改 shm_queue.h:92）
static constexpr size_t QUEUE_SIZE = 8192;  // 从4096改为8192
```

### 问题4：连接超时

**检查**：
```bash
# 检查模拟器
ps aux | grep md_simulator

# 检查Gateway
ps aux | grep md_gateway

# 检查端口
lsof -i :50051

# 检查共享内存
ls -lh /tmp/hft_md_*
```

## 🔧 开发调试

### 重新编译

**C++ Gateway：**
```bash
cd /Users/user/PWorks/RD/hft-poc
rm -rf gateway/build
./scripts/build_gateway.sh
```

**Golang客户端：**
```bash
cd /Users/user/PWorks/RD/hft-poc/golang
go build -o bin/md_client ./cmd/md_client
```

### 调试模式

**C++ Debug编译：**
```bash
cd gateway/build
cmake -DCMAKE_BUILD_TYPE=Debug ..
make
lldb ./md_gateway
```

**Golang调试：**
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/md_client -- -gateway localhost:50051
```

## 📊 性能测试

### 基准测试（推荐）
```bash
# 共享内存性能测试：10k Hz，持续30秒
./gateway/build/md_benchmark 10000 30
```

**预期结果：**
- 平均延迟: **~3.4 μs**
- P99延迟: **~9 μs**
- 吞吐量: **~10k msg/s**
- 丢包率: **0%**

### 完整集成测试
```bash
# NATS + 共享内存 + Gateway完整测试
./scripts/test_md_gateway_with_nats.sh
```

**预期结果：**
- Gateway发布: **15k+ 消息到NATS**
- NATS订阅: **接收100+ 消息**
- 处理延迟: **~30 μs**

## 📁 项目结构

```
hft-poc/
├── gateway/
│   ├── build/
│   │   ├── md_gateway      ← Gateway (共享内存模式)
│   │   ├── md_simulator        ← 行情模拟器
│   │   └── md_benchmark        ← 性能基准测试工具
│   ├── include/
│   │   ├── md_gateway.h        ← Gateway头文件
│   │   ├── shm_queue.h         ← 共享内存队列
│   │   └── performance_monitor.h ← 性能监控
│   ├── src/
│   │   ├── main_shm.cpp        ← Gateway主程序（共享内存）
│   │   ├── md_gateway.cpp      ← Gateway实现
│   │   ├── md_simulator.cpp    ← 模拟器实现
│   │   └── md_benchmark.cpp    ← 基准测试实现
│   └── proto/                  ← Protobuf定义
├── golang/
│   ├── bin/
│   │   └── md_client           ← Go 可执行文件
│   ├── cmd/md_client/          ← 客户端主程序
│   └── pkg/
│       ├── client/             ← 客户端库
│       └── proto/              ← 生成的Go代码
├── scripts/
│   ├── build_gateway.sh        ← 构建脚本
│   ├── test_md_gateway_with_nats.sh ← NATS集成测试
│   └── ...
├── QUICKSTART.md              ← 快速开始
├── SHM_EXAMPLE.md             ← 共享内存示例
├── PERFORMANCE_REPORT.md      ← 性能测试报告
├── README.md                  ← 项目说明
└── USAGE.md                   ← 本文档
```

## 🎯 下一步计划

### ✅ Week 1-4 已完成
- [x] POC环境搭建
- [x] MD Gateway实现（共享内存）
- [x] NATS集成
- [x] 性能测试工具

### 🚧 Week 5-6 进行中
- [ ] ORS Gateway（订单路由）
- [ ] 订单服务gRPC接口
- [ ] 订单回报推送

### 📋 Week 7+ 计划
- [ ] Counter Gateway（柜台对接）
- [ ] EES/CTP API封装
- [ ] Prometheus监控

## 💡 提示

1. **共享内存架构优势**：
   - 零拷贝IPC：延迟 <5μs
   - 进程隔离：故障不传播
   - 易于扩展：独立升级

2. **性能优化建议**：
   - ✅ 已使用共享内存（无需优化）
   - ✅ 已使用无锁队列（SPSC）
   - ✅ 已使用缓存行对齐

3. **监控建议**：
   - 使用 `md_benchmark` 定期测试
   - 监控队列利用率和丢包率
   - 设置延迟告警（如 P99 >50μs）

## 📞 技术支持

如有问题，请检查：
1. **日志输出**
   - Gateway: `/tmp/gateway.log`
   - 模拟器: `/tmp/simulator.log`

2. **共享内存状态**
   ```bash
   ls -lh /tmp/hft_md_*
   ```

3. **进程状态**
   ```bash
   ps aux | grep -E "md_gateway|md_simulator"
   ```

4. **详细文档**
   - [SHM_EXAMPLE.md](SHM_EXAMPLE.md) - 共享内存使用指南
   - [PERFORMANCE_REPORT.md](PERFORMANCE_REPORT.md) - 性能测试报告
   - [CLEANUP_SUMMARY.md](CLEANUP_SUMMARY.md) - 架构清理说明
