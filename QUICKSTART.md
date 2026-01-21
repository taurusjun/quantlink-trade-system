# 快速开始指南

## 最简单的启动方式（不需要NATS）

如果你只想快速测试gRPC功能，可以跳过NATS安装：

### 1. 安装基础依赖

```bash
brew install cmake protobuf grpc go
```

### 2. 编译

```bash
cd /Users/user/PWorks/RD/hft-poc

# 编译C++ Gateway（会自动检测NATS，没有也能编译）
./scripts/build_gateway.sh

# 编译Golang客户端
./scripts/build_golang.sh
```

### 3. 运行

**Terminal 1: 启动Gateway**
```bash
./gateway/build/md_gateway
```

你会看到：
```
╔═══════════════════════════════════════════════════════╗
║         HFT Market Data Gateway - POC v0.1           ║
║                                                       ║
║  gRPC: 0.0.0.0:50051                                 ║
║  NATS: nats://localhost:4222                         ║
╚═══════════════════════════════════════════════════════╝

[MDGateway] Started successfully
[MDGateway] NATS: Disabled    <-- 这是正常的
[MDGateway] gRPC server listening on 0.0.0.0:50051
```

**Terminal 2: 运行客户端**
```bash
./golang/bin/md_client -gateway localhost:50051 -symbols ag2412
```

你会看到实时行情输出：
```
[Client] Connected to gateway: localhost:50051
[Client] Subscribed to symbols: [ag2412]
[Client] Count: 100, Avg Latency: 234μs, Throughput: 98 msg/s
```

## 完整功能（包含NATS）

如果需要NATS推送功能：

### 1. 安装所有依赖

```bash
./scripts/install_dependencies.sh
```

### 2. 启动NATS服务器

```bash
nats-server &
```

### 3. 重新编译（启用NATS）

```bash
./scripts/build_gateway.sh
./scripts/build_golang.sh
```

### 4. 运行

**Terminal 1: Gateway**
```bash
./gateway/build/md_gateway
```

**Terminal 2: gRPC客户端**
```bash
./golang/bin/md_client -gateway localhost:50051 -symbols ag2412
```

**Terminal 3: NATS客户端**
```bash
./golang/bin/md_client -nats -symbols ag2412
```

## 常见问题

### Q: 编译时提示"NATS library not found"
A: 这是警告，不是错误。Gateway会禁用NATS功能但仍能正常编译运行。

### Q: 如何确认NATS是否启用？
A: 启动Gateway时看日志：
- `[MDGateway] NATS: Enabled` - NATS已启用
- `[MDGateway] NATS: Disabled` - NATS未启用（只有gRPC）

### Q: NATS未启用会影响功能吗？
A: 只影响NATS推送功能，gRPC流式推送功能完全正常。

### Q: 怎样重新编译以启用NATS？
A:
```bash
# 安装NATS C客户端
./scripts/install_nats_c.sh

# 清理并重新编译
rm -rf gateway/build
./scripts/build_gateway.sh
```

## 性能测试

```bash
# 运行10秒基准测试
timeout 10s ./golang/bin/md_client -gateway localhost:50051 -symbols ag2412
```

预期结果：
- 延迟: <1ms
- 吞吐量: >1000 msg/s
- CPU: <20%

## 下一步

- [ ] 查看完整文档: [README.md](README.md)
- [ ] 性能测试: [tests/](tests/)
- [ ] 配置调优: [config/](config/)
